package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// MatchRequest contains the parameters for matching skills.
type MatchRequest struct {
	PipelineID     string
	ProjectID      string
	Stage          string
	UserMessage    string
	PermissionMode string
	TokenBudget    int
	ForceSkill     string // "/skill <name>" forced pull
	MaxInject      int
}

// SkillLoader scans skill directories, parses Markdown+YAML frontmatter,
// merges runtime config, and provides CoW snapshot-based matching.
type SkillLoader struct {
	scanDirs []string // [global, team, project] in priority order
	mu       sync.Mutex
	current  atomic.Value // *skillSnapshot
	cache    sync.Map     // key: cacheKey → []Skill
	polling  *time.Ticker // 30s fallback
	stopCh   chan struct{}
}

// NewSkillLoader creates a SkillLoader, scans directories, and builds the initial snapshot.
func NewSkillLoader(scanDirs []string) (*SkillLoader, error) {
	sl := &SkillLoader{
		scanDirs: scanDirs,
		polling:  time.NewTicker(30 * time.Second),
		stopCh:   make(chan struct{}),
	}

	snapshot, err := sl.buildSnapshot()
	if err != nil {
		// Degrade: store empty snapshot, don't block Pipeline
		snapshot = &skillSnapshot{
			skills:    nil,
			config:    make(map[string]SkillConfig),
			buildTime: time.Now(),
		}
		sl.current.Store(snapshot)
		return sl, fmt.Errorf("SkillLoader init degraded (empty snapshot): %w", err)
	}
	sl.current.Store(snapshot)

	go sl.pollLoop()
	return sl, nil
}

// configKey returns a stable map key: "name@version"
func configKey(name, version string) string {
	return name + "@" + version
}

// buildSnapshot performs a full rebuild by scanning all directories.
func (sl *SkillLoader) buildSnapshot() (*skillSnapshot, error) {
	var allSkills []Skill
	config := make(map[string]SkillConfig)

	for i, dir := range sl.scanDirs {
		source := sl.dirSource(i) // "global" | "team" | "project"
		cfg, err := sl.loadConfigYAML(dir)
		if err != nil && !os.IsNotExist(err) {
			// config.yaml corrupt → WARN, continue from .md files
			cfg = make(map[string]SkillConfig)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("scan dir %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			skill, err := sl.parseSkillFile(filePath, source)
			if err != nil {
				// Skip malformed skill, log warning
				continue
			}

			// Merge runtime config from skill_config.yaml
			key := configKey(skill.Name, skill.Version)
			if sc, ok := cfg[key]; ok {
				skill.CurrentPriority = sc.CurrentPriority
				skill.Enabled = sc.Enabled
				skill.IsLatest = sc.IsLatest
				skill.Deprecated = sc.Deprecated
				if !sc.PublishedAt.IsZero() {
					skill.PublishedAt = sc.PublishedAt
				}
				if sc.BasePriority != 0 {
					skill.BasePriority = sc.BasePriority
				}
			} else {
				// No config entry: use defaults from frontmatter
				skill.CurrentPriority = float64(skill.BasePriority)
				skill.Enabled = true
				skill.IsLatest = true
			}

			allSkills = append(allSkills, skill)

			// Store config for snapshot
			config[key] = SkillConfig{
				Name:            skill.Name,
				Version:         skill.Version,
				File:            entry.Name(),
				BasePriority:    skill.BasePriority,
				CurrentPriority: skill.CurrentPriority,
				Enabled:         skill.Enabled,
				IsLatest:        skill.IsLatest,
				Deprecated:      skill.Deprecated,
				PublishedAt:     skill.PublishedAt,
			}
		}
	}

	return &skillSnapshot{
		skills:    allSkills,
		config:    config,
		buildTime: time.Now(),
	}, nil
}

func (sl *SkillLoader) dirSource(idx int) string {
	switch idx {
	case 0:
		return "global"
	case 1:
		return "team"
	case 2:
		return "project"
	default:
		return "unknown"
	}
}

func (sl *SkillLoader) loadConfigYAML(dir string) (map[string]SkillConfig, error) {
	path := filepath.Join(dir, "skill_config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Skills []SkillConfig `yaml:"skills"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	result := make(map[string]SkillConfig, len(wrapper.Skills))
	for _, sc := range wrapper.Skills {
		result[configKey(sc.Name, sc.Version)] = sc
	}
	return result, nil
}

// parseSkillFile parses a Markdown file with YAML frontmatter.
func (sl *SkillLoader) parseSkillFile(path, source string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}

	content := string(data)
	skill := Skill{
		Source:   source,
		FilePath: path,
	}

	// Parse YAML frontmatter between --- delimiters
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		rest := content[4:] // skip "---\n"
		endIdx := strings.Index(rest, "\n---")
		if endIdx < 0 {
			rest = content[3:] // try "---\n"
			endIdx = strings.Index(rest, "\n---")
		}
		if endIdx >= 0 {
			frontmatter := rest[:endIdx]
			if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
				return Skill{}, fmt.Errorf("parse frontmatter in %s: %w", path, err)
			}
			body := rest[endIdx+4:] // skip "\n---"
			sl.parseBody(&skill, body)
		}
	}

	// Set defaults
	if skill.BasePriority == 0 {
		skill.BasePriority = DefaultBasePriority
	}
	skill.CreatedAt = time.Now()

	return skill, nil
}

// parseBody extracts Prompt and Workflow sections from Markdown body.
func (sl *SkillLoader) parseBody(skill *Skill, body string) {
	parts := strings.Split(body, "\n## ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Prompt") || strings.HasPrefix(part, "Prompt\n") {
			// Content after "Prompt" header line
			lines := strings.SplitN(part, "\n", 2)
			if len(lines) == 2 {
				skill.Prompt = strings.TrimSpace(lines[1])
			}
		}
		if strings.HasPrefix(part, "Workflow") || strings.HasPrefix(part, "Workflow\n") {
			lines := strings.SplitN(part, "\n", 2)
			if len(lines) == 2 {
				for _, line := range strings.Split(lines[1], "\n") {
					line = strings.TrimSpace(line)
					if line != "" && (strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "-") ||
						strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") ||
						strings.HasPrefix(line, "4.") || strings.HasPrefix(line, "5.")) {
						// Strip leading number/bullet
						cleaned := strings.TrimLeft(line, "0123456789.- ")
						if cleaned != "" {
							skill.Workflow = append(skill.Workflow, cleaned)
						}
					}
				}
			}
		}
	}
}

// cacheKey generates a cache key from MatchRequest parameters.
func (sl *SkillLoader) cacheKey(req MatchRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Stage))
	h.Write([]byte(req.UserMessage))
	h.Write([]byte(fmt.Sprintf("%d", req.TokenBudget)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Match filters, scores, and ranks skills for the given request.
func (sl *SkillLoader) Match(req MatchRequest) []Skill {
	// Force skill via /skill command
	if req.ForceSkill != "" {
		skill, err := sl.MatchByName(req.ForceSkill)
		if err != nil || skill == nil {
			return nil
		}
		return []Skill{*skill}
	}

	maxInject := req.MaxInject
	if maxInject <= 0 {
		maxInject = DefaultMaxInject
	}

	// Check cache
	ck := sl.cacheKey(req)
	if cached, ok := sl.cache.Load(ck); ok {
		return cached.([]Skill)
	}

	snapshot := sl.current.Load().(*skillSnapshot)
	results := sl.matchFromSnapshot(snapshot, req, maxInject)

	sl.cache.Store(ck, results)
	return results
}

type skillCandidate struct {
	skill          Skill
	finalScore     float64
	triggerScore   float64
	triggeredBy    string
	levelScore     int
}

func (sl *SkillLoader) matchFromSnapshot(snapshot *skillSnapshot, req MatchRequest, maxInject int) []Skill {
	// Stage exact filtering
	var candidates []skillCandidate
	for _, skill := range snapshot.skills {
		if !sl.stageMatches(skill, req.Stage) {
			continue
		}
		if skill.Deprecated {
			continue
		}
		if !skill.Enabled {
			continue
		}
		if !sl.permissionMatches(skill, req.PermissionMode) {
			continue
		}

		// Calculate trigger score
		triggerPoints, triggeredBy := sl.calcTriggerPoints(skill, req.UserMessage)
		matchScore := triggerPoints / 50.0
		if matchScore > 1.0 {
			matchScore = 1.0
		}
		finalScore := skill.CurrentPriority * matchScore

		candidates = append(candidates, skillCandidate{
			skill:        skill,
			finalScore:   finalScore,
			triggerScore: triggerPoints,
			triggeredBy:  triggeredBy,
			levelScore:   sl.sourceLevel(skill.Source),
		})
	}

	// All deprecated → return empty
	if len(candidates) == 0 {
		allDeprecated := true
		for _, skill := range snapshot.skills {
			if sl.stageMatches(skill, req.Stage) && !skill.Deprecated {
				allDeprecated = false
				break
			}
		}
		if allDeprecated {
			return nil // WARN logged by caller
		}
		return nil
	}

	// Deduplicate by name: keep highest CurrentPriority version
	bestByName := make(map[string]skillCandidate)
	for _, c := range candidates {
		existing, ok := bestByName[c.skill.Name]
		if !ok || c.skill.CurrentPriority > existing.skill.CurrentPriority {
			bestByName[c.skill.Name] = c
		}
	}

	var deduped []skillCandidate
	for _, c := range bestByName {
		deduped = append(deduped, c)
	}

	// Sort: finalScore desc → source level → CurrentPriority → version → createdAt → name
	sort.Slice(deduped, func(i, j int) bool {
		a, b := deduped[i], deduped[j]
		if a.finalScore != b.finalScore {
			return a.finalScore > b.finalScore
		}
		if a.levelScore != b.levelScore {
			return a.levelScore > b.levelScore // project(3) > team(2) > global(1)
		}
		if a.skill.CurrentPriority != b.skill.CurrentPriority {
			return a.skill.CurrentPriority > b.skill.CurrentPriority
		}
		if a.skill.Version != b.skill.Version {
			return a.skill.Version > b.skill.Version // semver string compare works for simple versions
		}
		if !a.skill.CreatedAt.Equal(b.skill.CreatedAt) {
			return a.skill.CreatedAt.After(b.skill.CreatedAt)
		}
		return a.skill.Name < b.skill.Name
	})

	// Top-K truncate
	if len(deduped) > maxInject {
		deduped = deduped[:maxInject]
	}

	result := make([]Skill, len(deduped))
	for i, c := range deduped {
		result[i] = c.skill
	}
	return result
}

// MatchByName finds a skill by name (for /skill command), ignoring stage/permission.
func (sl *SkillLoader) MatchByName(name string) (*Skill, error) {
	snapshot := sl.current.Load().(*skillSnapshot)

	var best *Skill
	for i := range snapshot.skills {
		skill := &snapshot.skills[i]
		if skill.Name == name && skill.Enabled && !skill.Deprecated {
			if best == nil || skill.CurrentPriority > best.CurrentPriority {
				best = skill
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return best, nil
}

func (sl *SkillLoader) stageMatches(skill Skill, stage string) bool {
	for _, s := range skill.Stages {
		if s == stage {
			return true
		}
	}
	return false
}

func (sl *SkillLoader) permissionMatches(skill Skill, mode string) bool {
	if len(skill.Permission) == 0 {
		return true // no restriction = all modes
	}
	if mode == "" {
		return true // request doesn't specify mode = allow all
	}
	for _, p := range skill.Permission {
		if p == mode {
			return true
		}
	}
	return false
}

func (sl *SkillLoader) calcTriggerPoints(skill Skill, userMsg string) (float64, string) {
	points := 0.0
	var reasons []string
	lowerMsg := strings.ToLower(userMsg)

	// File pattern matching (highest weight: +30)
	for _, pattern := range skill.Triggers.FilePatterns {
		if matchPattern(lowerMsg, strings.ToLower(pattern)) {
			points += 30
			reasons = append(reasons, "file_pattern")
			break
		}
	}

	// User intent matching (+20)
	for _, intent := range skill.Triggers.UserIntent {
		if strings.Contains(lowerMsg, strings.ToLower(intent)) {
			points += 20
			reasons = append(reasons, "user_intent")
			break
		}
	}

	// Keyword matching (+15 × hit rate)
	keywordHits := 0
	for _, kw := range skill.Keywords {
		if strings.Contains(lowerMsg, strings.ToLower(kw)) {
			keywordHits++
		}
	}
	if len(skill.Keywords) > 0 && keywordHits > 0 {
		hitRate := float64(keywordHits) / float64(len(skill.Keywords))
		points += 15 * hitRate
		if len(reasons) == 0 {
			reasons = append(reasons, "keyword_match")
		}
	}

	if len(reasons) == 0 {
		return 0, "stage_filter"
	}
	return points, reasons[0]
}

func (sl *SkillLoader) sourceLevel(source string) int {
	switch source {
	case "project":
		return 3
	case "team":
		return 2
	case "global":
		return 1
	default:
		return 0
	}
}

// Reload performs a full snapshot rebuild. Old snapshot remains readable during rebuild.
func (sl *SkillLoader) Reload() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	snapshot, err := sl.buildSnapshot()
	if err != nil {
		// Keep old snapshot on failure
		return fmt.Errorf("reload failed, keeping old snapshot: %w", err)
	}

	sl.current.Store(snapshot)
	// Clear cache — old cache references old snapshot
	sl.cache = sync.Map{}
	return nil
}

// GetSnapshot returns the current snapshot (for admin APIs).
func (sl *SkillLoader) GetSnapshot() *skillSnapshot {
	return sl.current.Load().(*skillSnapshot)
}

// GetAllSkills returns all skills in the current snapshot.
func (sl *SkillLoader) GetAllSkills() []Skill {
	snapshot := sl.current.Load().(*skillSnapshot)
	result := make([]Skill, len(snapshot.skills))
	copy(result, snapshot.skills)
	return result
}

func (sl *SkillLoader) pollLoop() {
	for {
		select {
		case <-sl.polling.C:
			_ = sl.Reload()
		case <-sl.stopCh:
			sl.polling.Stop()
			return
		}
	}
}

// Stop stops the polling loop.
func (sl *SkillLoader) Stop() {
	close(sl.stopCh)
}

// matchPattern checks if user message or file path matches a glob-like pattern.
func matchPattern(text, pattern string) bool {
	// Simple glob matching: * matches any sequence
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		// Extension match: "*.tsx"
		ext := pattern[1:] // ".tsx"
		return strings.HasSuffix(text, ext)
	}
	if strings.Contains(pattern, "/") {
		// Path-like pattern
		return strings.Contains(text, strings.ReplaceAll(pattern, "*", ""))
	}
	return strings.Contains(text, pattern)
}
