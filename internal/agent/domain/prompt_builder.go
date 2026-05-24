// Package domain provides the core domain logic for the OpenForge agent.
// This file implements the PromptBuilder, which constructs structured prompts
// for LLM interactions based on Pipeline stage, complexity level, and permissions.
package domain

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	agentport "openforge/internal/agent/port"
)

// Message is a type alias to avoid conversion between domain and port.
type Message = agentport.Message

// PromptBuilder constructs structured prompts for LLM interactions.
// Architecture: L1 (static.xml) → L2 (project fusion) → Capability (Skill+Tool) → L4 (conversation).
type PromptBuilder struct {
	l1Content    string          // static.xml in-memory cache, loaded at startup
	l2Builder    *L2Builder
	capInjector  *CapabilityInjector
	metrics      *PromptMetrics
	mu           sync.RWMutex
}

// L2Builder handles all L2 content assembly.
type L2Builder struct {
	prefs     *ProjectPrefsLoader
	knowledge *KnowledgeQuerier
	mu        sync.RWMutex
}

// L2Request holds the inputs needed by L2Builder.
type L2Request struct {
	ProjectID        string
	PipelineID       string
	Stage            string
	Level            string
	UserQuery        string
	BacktrackReason  string
	BacktrackTarget  string
	ParentPipelineID string
}

// Prompt is the build output — only System text + Tools + Token stats.
// Messages are managed independently by QueryEngine.
type Prompt struct {
	System     string
	Tools      []ToolDefinition
	TokenUsage *TokenUsage
}

// PromptConfig configuration for prompt building
type PromptConfig struct {
	ModelAlias          string
	MaxTokens           int
	Temperature         float64
	CacheEnabled        bool
	CacheTTL            time.Duration
	SanitizationEnabled bool
	InjectionDefense    bool
	AuditLogging        bool
	TokenBudget         *TokenBudget
}

// TokenBudget defines token limits for different layers
type TokenBudget struct {
	Total        int
	Static       int
	Project      int
	Stage        int
	Conversation int
	Knowledge    int
	Tools        int
}

// DefaultTokenBudget returns default token budget
func DefaultTokenBudget() *TokenBudget {
	return &TokenBudget{
		Total:        10500,
		Static:       2000,
		Project:      1500,
		Stage:        2000,
		Conversation: 3000,
		Knowledge:    1000,
		Tools:        1000,
	}
}

// BuildRequest contains all information needed to build a prompt
type BuildRequest struct {
	PipelineID     string
	ProjectID      string
	Stage          string
	StageLevel     string
	PermissionMode string
	UserRole       string
	UserMessage    string

	ConversationHistory []Message

	BacktrackReason   string
	BacktrackTarget   string
	BacktrackEvidence string

	ParentPipelineID string
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	StaticTokens       int
	ConversationTokens int
	ToolTokens         int
	TotalTokens        int
}

// PromptMetrics contains metrics about prompt building
type PromptMetrics struct {
	BuildDuration   time.Duration
	BuildSuccess    bool
	BuildError      string
	L1CacheHit      bool
	L2CacheHit      bool
	L4CacheHit      bool
	TotalTokens     int
	Stage           string
	ComplexityLevel string
	PermissionMode  string
}

// StageTemplate represents a stage-specific template
type StageTemplate struct {
	Stage      string
	Complexity string
	Template   string
}

// TemplateData contains data for template filling (retained for compatibility)
type TemplateData struct {
	Static       string
	Project      string
	Stage        string
	Conversation string
	Knowledge    string
	Tools        string
	Config       *BuildRequest
}

// ============================================================
// Constructor
// ============================================================

// NewPromptBuilder creates a PromptBuilder.
// l1Path is the path to config/prompts/static.xml.
// knowledgeQuerier may be nil (Phase 7: real LearningEngine).
func NewPromptBuilder(l1Path string, knowledgeQuerier *KnowledgeQuerier) (*PromptBuilder, error) {
	l1Content, err := os.ReadFile(l1Path)
	if err != nil {
		return nil, fmt.Errorf("read static.xml: %w", err)
	}
	return &PromptBuilder{
		l1Content: string(l1Content),
		l2Builder: &L2Builder{
			prefs:     NewProjectPrefsLoader(30 * time.Second),
			knowledge: knowledgeQuerier,
		},
	}, nil
}

// ============================================================
// Core Build
// ============================================================

// Build constructs a complete prompt for LLM interaction.
func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
	startTime := time.Now()

	pb.mu.Lock()
	pb.metrics = &PromptMetrics{
		Stage:           req.Stage,
		ComplexityLevel: req.StageLevel,
		PermissionMode:  req.PermissionMode,
	}
	pb.mu.Unlock()

	// 1. L2 build
	l2Content, err := pb.l2Builder.Build(ctx, &L2Request{
		ProjectID:        req.ProjectID,
		PipelineID:       req.PipelineID,
		Stage:            req.Stage,
		Level:            req.StageLevel,
		UserQuery:        req.UserMessage,
		BacktrackReason:  req.BacktrackReason,
		BacktrackTarget:  req.BacktrackTarget,
		ParentPipelineID: req.ParentPipelineID,
	})
	if err != nil {
		return nil, fmt.Errorf("l2 build: %w", err)
	}

	// 2. Capability layer: Skill + Tool unified injection
	var capXML string
	var toolDefs []ToolDefinition
	if pb.capInjector != nil {
		capResult, err := pb.capInjector.Inject(ctx, CapabilityRequest{
			PipelineID:     req.PipelineID,
			ProjectID:      req.ProjectID,
			Stage:          req.Stage,
			UserMessage:    req.UserMessage,
			PermissionMode: req.PermissionMode,
			TokenBudget:    100000, // Phase 7: from BuildRequest.TokenBudget
		})
		if err == nil && capResult != nil {
			capXML = pb.capInjector.BuildCapabilityXML(capResult)
			for _, t := range capResult.Tools {
				toolDefs = append(toolDefs, ToolDefinition{
					Name:        t.Name,
					Description: t.Description,
					ReadOnly:    t.ReadOnly,
					InputSchema: t.InputSchema,
				})
			}
		}
	}
	// Fallback: if CapabilityInjector not set, use hardcoded tools
	if len(toolDefs) == 0 {
		_, toolDefs = InjectTools(req.Stage, req.PermissionMode)
	}

	// 3. L4 conversation summary
	l4Summary := buildL4Summary(req.ConversationHistory)

	// 4. Assemble System: L2 + Capability + L4Summary + L1 (L1 last, non-overridable)
	systemPrompt := strings.Join([]string{l2Content, capXML, l4Summary, pb.l1Content}, "\n")

	// 5. Security sanitization
	systemPrompt = sanitizePrompt(systemPrompt)

	// 6. Token estimation
	tokenUsage := calcTokenUsage(systemPrompt, req.ConversationHistory, toolDefs)

	pb.mu.Lock()
	pb.metrics.BuildDuration = time.Since(startTime)
	pb.metrics.TotalTokens = tokenUsage.TotalTokens
	pb.metrics.BuildSuccess = true
	pb.mu.Unlock()

	return &Prompt{
		System:     systemPrompt,
		Tools:      toolDefs,
		TokenUsage: tokenUsage,
	}, nil
}

// GetMetrics returns current metrics
func (pb *PromptBuilder) GetMetrics() *PromptMetrics {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.metrics
}

// SetCapabilityInjector sets the CapabilityInjector for Skill+Tool unified injection.
func (pb *PromptBuilder) SetCapabilityInjector(ci *CapabilityInjector) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.capInjector = ci
}

// ============================================================
// L2Builder
// ============================================================

func (l2 *L2Builder) Build(ctx context.Context, req *L2Request) (string, error) {
	var parts []string

	// 1. Project preferences (Phase 5d: returns empty, Phase 6: reads of-prefs.yaml)
	if prefs := l2.prefs.Get(req.ProjectID); prefs != "" {
		parts = append(parts, prefs)
	}

	// 2. Stage instruction (project override → server template → generic fallback)
	parts = append(parts, l2.stageInstruction(req.Stage, req.Level, req.ProjectID))

	// 3. Knowledge query (silently returns empty when learningEngine is nil)
	if l2.knowledge != nil {
		if knowledge, err := l2.knowledge.Query(ctx, req.ProjectID, req.UserQuery); err == nil && knowledge != "" {
			parts = append(parts, knowledge)
		}
	}

	// 4. Metadata
	parts = append(parts, l2.metadata(req))

	return strings.Join(parts, "\n"), nil
}

func (l2 *L2Builder) stageInstruction(stage, level, projectID string) string {
	// 1. Check of-prefs.yaml project override
	if override := l2.prefs.GetStageOverride(projectID, stage, level); override != "" {
		return override
	}
	// 2. Fallback to server template
	if tmpl := GetStageTemplate(stage, level); tmpl != nil {
		return tmpl.Template
	}
	// 3. Generic fallback
	return fmt.Sprintf("<stage_instructions stage=\"%s\" level=\"%s\">Execute according to Pipeline rules.</stage_instructions>", stage, level)
}

func (l2 *L2Builder) metadata(req *L2Request) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("<pipeline_id>%s</pipeline_id>", req.PipelineID))
	parts = append(parts, fmt.Sprintf("<project_id>%s</project_id>", req.ProjectID))
	parts = append(parts, fmt.Sprintf("<current_time>%s</current_time>", time.Now().Format(time.RFC3339)))

	if req.BacktrackReason != "" {
		parts = append(parts, fmt.Sprintf("<backtrack_context reason=\"%s\" target=\"%s\"/>", req.BacktrackReason, req.BacktrackTarget))
	}
	if req.ParentPipelineID != "" {
		parts = append(parts, fmt.Sprintf("<parent_pipeline id=\"%s\"/>", req.ParentPipelineID))
	}

	return "<context>\n" + strings.Join(parts, "\n") + "\n</context>"
}

// ============================================================
// ProjectPrefsLoader (Phase 5d: returns defaults, Phase 6: of-prefs.yaml + mtime hot-reload)
// ============================================================

// ProjectPrefsLoader loads project preferences.
type ProjectPrefsLoader struct {
	interval time.Duration
	mu       sync.RWMutex
}

// NewProjectPrefsLoader creates a new ProjectPrefsLoader.
func NewProjectPrefsLoader(interval time.Duration) *ProjectPrefsLoader {
	return &ProjectPrefsLoader{interval: interval}
}

// Get returns project preference content (Phase 5d: empty, Phase 6: reads file + cache + hot-reload).
func (pl *ProjectPrefsLoader) Get(projectID string) string {
	return "" // Phase 6
}

// GetStageOverride returns project-overridden stage instruction (Phase 5d: empty).
func (pl *ProjectPrefsLoader) GetStageOverride(projectID, stage, level string) string {
	return "" // Phase 6
}

// ============================================================
// L4 pure functions
// ============================================================

// buildL4Summary builds a conversation summary for the SystemPrompt.
// Messages arrays are fully managed by QueryEngine independently.
func buildL4Summary(history []Message) string {
	if len(history) == 0 {
		return ""
	}
	recent := history
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	var b strings.Builder
	b.WriteString("<conversation_summary>\n")
	for i, msg := range recent {
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("<msg seq=\"%d\" role=\"%s\">%s</msg>\n", i+1, msg.Role, content))
	}
	b.WriteString("</conversation_summary>")
	return b.String()
}

// ============================================================
// Security sanitization (fixes former removePattern no-op)
// ============================================================

func sanitizePrompt(content string) string {
	patterns := []string{
		"SYSTEM:", "指令", "you are now",
		"ignore previous", "disregard instructions",
	}
	for _, p := range patterns {
		content = strings.ReplaceAll(content, p, "")
	}
	return content
}

// ============================================================
// Token estimation
// ============================================================

func calcTokenUsage(system string, messages []Message, tools []ToolDefinition) *TokenUsage {
	usage := &TokenUsage{}
	usage.StaticTokens = len(system) / 4
	for _, msg := range messages {
		usage.ConversationTokens += len(msg.Content) / 4
	}
	for _, t := range tools {
		usage.ToolTokens += len(t.Description) / 4
	}
	usage.TotalTokens = usage.StaticTokens + usage.ConversationTokens + usage.ToolTokens
	return usage
}
