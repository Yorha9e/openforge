package domain

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// UnifiedPriorityEngine recalculates skill priorities daily and manages deprecation.
type UnifiedPriorityEngine struct {
	skillLoader *SkillLoader
	config      *SkillEngineConfig
	mu          sync.Mutex
	stopCh      chan struct{}
}

// SkillEngineConfig holds priority engine configuration.
type SkillEngineConfig struct {
	Skill struct {
		MaxInject      int            `yaml:"max_inject"`
		AvgSkillTokens int            `yaml:"avg_skill_tokens"`
		Priority       PriorityConfig `yaml:"priority"`
		Watch          WatchConfig    `yaml:"watch"`
	} `yaml:"skill"`
}

// PriorityConfig defines the priority algorithm parameters.
type PriorityConfig struct {
	DefaultBase         float64 `yaml:"default_base"`
	DecayMode           string  `yaml:"decay_mode"`
	DecayRate           float64 `yaml:"decay_rate"`
	MaxDecayDays        int     `yaml:"max_decay_days"`
	MinVersionFactor    float64 `yaml:"min_version_factor"`
	WindowDays          int     `yaml:"window_days"`
	AcceptWeight        float64 `yaml:"accept_weight"`
	UsageWeight         float64 `yaml:"usage_weight"`
	DeprecateThreshold  float64 `yaml:"deprecate_threshold"`
	MinUsageForLearning int     `yaml:"min_usage_for_learning"`
}

// WatchConfig defines file watch configuration.
type WatchConfig struct {
	DebounceMs    int `yaml:"debounce_ms"`
	PollFallbackS int `yaml:"poll_fallback_s"`
}

// DailyUpdateMetrics holds aggregated metrics for priority calculation.
type DailyUpdateMetrics struct {
	SkillName            string
	AcceptRate7d         float64
	RecentUsageCount     int
	RecentTotalDecisions int
}

// priorityUpdate describes a priority change for a skill.
type priorityUpdate struct {
	name            string
	version         string
	oldPriority     float64
	newPriority     float64
	shouldDeprecate bool
}

// DefaultSkillEngineConfig returns the default configuration.
func DefaultSkillEngineConfig() *SkillEngineConfig {
	cfg := &SkillEngineConfig{}
	cfg.Skill.MaxInject = 5
	cfg.Skill.AvgSkillTokens = 500
	cfg.Skill.Priority.DefaultBase = 70
	cfg.Skill.Priority.DecayMode = "exponential"
	cfg.Skill.Priority.DecayRate = 0.1
	cfg.Skill.Priority.MaxDecayDays = 60
	cfg.Skill.Priority.MinVersionFactor = 0.05
	cfg.Skill.Priority.WindowDays = 7
	cfg.Skill.Priority.AcceptWeight = 0.7
	cfg.Skill.Priority.UsageWeight = 0.3
	cfg.Skill.Priority.DeprecateThreshold = 0.15
	cfg.Skill.Priority.MinUsageForLearning = 10
	cfg.Skill.Watch.DebounceMs = 1000
	cfg.Skill.Watch.PollFallbackS = 30
	return cfg
}

// NewUnifiedPriorityEngine creates a new priority engine.
func NewUnifiedPriorityEngine(skillLoader *SkillLoader, config *SkillEngineConfig) *UnifiedPriorityEngine {
	if config == nil {
		config = DefaultSkillEngineConfig()
	}
	return &UnifiedPriorityEngine{
		skillLoader: skillLoader,
		config:      config,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the daily priority update loop.
func (upe *UnifiedPriorityEngine) Start() {
	go upe.dailyLoop()
}

// Stop stops the daily loop.
func (upe *UnifiedPriorityEngine) Stop() {
	close(upe.stopCh)
}

func (upe *UnifiedPriorityEngine) dailyLoop() {
	for {
		next := nextDailyRun(3, 0) // 03:00
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			if err := upe.RunDailyUpdate(); err != nil {
				_ = err
			}
		case <-upe.stopCh:
			timer.Stop()
			return
		}
	}
}

// RunDailyUpdate performs the daily priority recalculation.
func (upe *UnifiedPriorityEngine) RunDailyUpdate() error {
	upe.mu.Lock()
	defer upe.mu.Unlock()

	skills := upe.skillLoader.GetAllSkills()
	cfg := upe.config.Skill.Priority

	latestPublishBySkill := make(map[string]time.Time)
	for _, s := range skills {
		if s.IsLatest {
			latestPublishBySkill[s.Name] = s.PublishedAt
		}
	}

	var updates []priorityUpdate
	updateCount := 0
	deprecationCount := 0

	for _, skill := range skills {
		oldP := skill.CurrentPriority
		newP := upe.calculatePriority(skill, latestPublishBySkill, cfg)

		if newP != oldP {
			shouldDep := newP < cfg.DeprecateThreshold
			updates = append(updates, priorityUpdate{
				name:            skill.Name,
				version:         skill.Version,
				oldPriority:     oldP,
				newPriority:     newP,
				shouldDeprecate: shouldDep,
			})
			updateCount++
			if shouldDep {
				deprecationCount++
			}
		}
	}

	_ = updateCount
	_ = deprecationCount

	if len(updates) > 0 {
		for _, dir := range upe.skillLoader.scanDirs {
			_ = upe.writePriorityUpdates(dir, updates)
		}
		_ = upe.skillLoader.Reload()
	}

	return nil
}

func (upe *UnifiedPriorityEngine) calculatePriority(skill Skill, latestPublish map[string]time.Time, cfg PriorityConfig) float64 {
	basePriority := float64(skill.BasePriority)
	if basePriority == 0 {
		basePriority = cfg.DefaultBase
	}

	versionFactor := 1.0
	if !skill.IsLatest {
		if latestTime, ok := latestPublish[skill.Name]; ok {
			daysSince := time.Since(latestTime).Hours() / 24
			versionFactor = math.Exp(-cfg.DecayRate * daysSince)
			if versionFactor < cfg.MinVersionFactor {
				versionFactor = cfg.MinVersionFactor
			}
		}
	}

	learningFactor := 1.0
	return basePriority * versionFactor * learningFactor
}

func (upe *UnifiedPriorityEngine) writePriorityUpdates(dir string, updates []priorityUpdate) error {
	path := filepath.Join(dir, "skill_config.yaml")

	var wrapper struct {
		Skills []SkillConfig `yaml:"skills"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	updateMap := make(map[string]priorityUpdate)
	for _, u := range updates {
		updateMap[u.name+"@"+u.version] = u
	}

	changed := false
	for i, sc := range wrapper.Skills {
		key := sc.Name + "@" + sc.Version
		if u, ok := updateMap[key]; ok {
			wrapper.Skills[i].CurrentPriority = u.newPriority
			wrapper.Skills[i].Deprecated = u.shouldDeprecate
			if u.shouldDeprecate {
				wrapper.Skills[i].Enabled = false
			}
			changed = true
		}
	}

	if !changed {
		return nil
	}

	newData, err := yaml.Marshal(&wrapper)
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func nextDailyRun(hour, minute int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
