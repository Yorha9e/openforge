package domain

import (
	"context"
	"fmt"
	"strings"
)

// ToolRegistryClient is the interface for tool search (backed by ToolRegistry in Phase 7,
// uses hardcoded StageToolMap in Phase 1-6).
type ToolRegistryClient interface {
	SearchTools(ctx context.Context, stage string, topK int) ([]ToolDefinition, error)
}

// CapabilityInjector unifies Skill + Tool injection as an intermediate capability layer
// between L2 (project fusion) and L4 (conversation summary) in the PromptBuilder chain.
type CapabilityInjector struct {
	skillLoader  *SkillLoader
	toolRegistry ToolRegistryClient
}

// CapabilityRequest contains the inputs for capability injection.
type CapabilityRequest struct {
	PipelineID     string
	ProjectID      string
	Stage          string
	UserMessage    string
	PermissionMode string
	TokenBudget    int
	ForceSkill     string
}

// CapabilityResult holds the injection output.
type CapabilityResult struct {
	Skills      []SkillInjectionRecord
	Tools       []ToolInjectionRecord
	PendingUser bool
	Message     string
}

// ToolInjectionRecord records a tool that was injected into a prompt.
type ToolInjectionRecord struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ReadOnly    bool                   `json:"read_only"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// NewCapabilityInjector creates a new CapabilityInjector.
func NewCapabilityInjector(skillLoader *SkillLoader, toolRegistry ToolRegistryClient) *CapabilityInjector {
	return &CapabilityInjector{
		skillLoader:  skillLoader,
		toolRegistry: toolRegistry,
	}
}

// Inject runs Skill matching + Tool injection, returning a unified capability result.
func (ci *CapabilityInjector) Inject(ctx context.Context, req CapabilityRequest) (*CapabilityResult, error) {
	result := &CapabilityResult{}

	// 1. Skill matching
	if ci.skillLoader != nil {
		skills := ci.skillLoader.Match(MatchRequest{
			PipelineID:     req.PipelineID,
			ProjectID:      req.ProjectID,
			Stage:          req.Stage,
			UserMessage:    req.UserMessage,
			PermissionMode: req.PermissionMode,
			TokenBudget:    req.TokenBudget,
			ForceSkill:     req.ForceSkill,
			MaxInject:      DefaultMaxInject,
		})

		// Token budget check
		if req.TokenBudget == 0 && len(skills) > 0 {
			result.PendingUser = true
			result.Message = fmt.Sprintf("%d skills matched. Inject into prompt? [Yes/Skip]", len(skills))
			return result, nil
		}

		for _, skill := range skills {
			record := SkillInjectionRecord{
				Name:            skill.Name,
				Version:         skill.Version,
				Source:          skill.Source,
				CurrentPriority: skill.CurrentPriority,
				TokenCost:       len(skill.Prompt) / 4, // rough estimation
			}
			result.Skills = append(result.Skills, record)
		}
	}

	// 2. Tool injection
	if ci.toolRegistry != nil {
		tools, err := ci.toolRegistry.SearchTools(ctx, req.Stage, 10)
		if err != nil {
			// Non-fatal: tools degrade, skills still injected
			result.Message = fmt.Sprintf("tool search degraded: %v", err)
		}
		for _, t := range tools {
			result.Tools = append(result.Tools, ToolInjectionRecord{
				Name:        t.Name,
				Description: t.Description,
				ReadOnly:    t.ReadOnly,
				InputSchema: t.InputSchema,
			})
		}
	}

	return result, nil
}

// BuildCapabilityXML formats the capability layer as XML for SystemPrompt injection.
func (ci *CapabilityInjector) BuildCapabilityXML(result *CapabilityResult) string {
	var parts []string

	if len(result.Skills) > 0 {
		var b strings.Builder
		b.WriteString("<capabilities>\n")
		b.WriteString("<skills>\n")
		for _, rec := range result.Skills {
			b.WriteString(fmt.Sprintf(
				"<skill name=\"%s\" version=\"%s\" source=\"%s\" trigger_score=\"%.1f\">\n",
				rec.Name, rec.Version, rec.Source, rec.TriggerScore,
			))
			b.WriteString("</skill>\n")
		}
		b.WriteString("</skills>\n")
		b.WriteString("</capabilities>\n")
		parts = append(parts, b.String())
	}

	if len(result.Tools) > 0 {
		var b strings.Builder
		b.WriteString("<tools>\n")
		for _, rec := range result.Tools {
			b.WriteString(fmt.Sprintf("<tool name=\"%s\" description=\"%s\" readonly=\"%v\"/>\n",
				rec.Name, rec.Description, rec.ReadOnly))
		}
		b.WriteString("</tools>\n")
		parts = append(parts, b.String())
	}

	return strings.Join(parts, "\n")
}

// HardcodedToolRegistry implements ToolRegistryClient using StageToolMap.
type HardcodedToolRegistry struct{}

var stageAliases = map[string]string{
	"impl":       "implement",
	"decompose":  "decompose",
	"clarify":    "clarify",
	"test":       "test",
	"deploy":     "deploy",
	"verify":     "verify",
	"implement":  "implement",
}

func (h *HardcodedToolRegistry) SearchTools(ctx context.Context, stage string, topK int) ([]ToolDefinition, error) {
	// Resolve abbreviation aliases
	if resolved, ok := stageAliases[stage]; ok {
		stage = resolved
	}
	tools, ok := StageToolMap[stage]
	if !ok {
		return nil, nil
	}
	if topK > 0 && len(tools) > topK {
		tools = tools[:topK]
	}
	return tools, nil
}
