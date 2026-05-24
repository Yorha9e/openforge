package domain

import (
	"fmt"
	"time"

	auth "openforge/internal/auth/domain"
)

type Pipeline struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	Title            string         `json:"title"`
	Level            string         `json:"level"`
	Status           string         `json:"status"`
	CurrentStage     string         `json:"current_stage"`
	CreatedBy        string         `json:"created_by"`
	BacktrackCount   int            `json:"backtrack_count"`
	Version          int            `json:"version"`
	Stages           []Stage        `json:"stages"`
	ParentPipelineID *string        `json:"parent_pipeline_id,omitempty"`
	Region           string         `json:"region"`
	Config           PipelineConfig `json:"config"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// PipelineConfig holds the configuration snapshot for a pipeline.
type PipelineConfig struct {
	Language  string `json:"language"`
	Framework string `json:"framework"`
	MaxAgents int    `json:"max_agents"`
}

func (c PipelineConfig) Clone() PipelineConfig { return c }

func NewPipeline(id, projectID, title, createdBy string, files, modules int) *Pipeline {
	level := ClassifyComplexity(files, modules)
	stages := defaultStages(level)
	p := &Pipeline{
		ID:        id,
		ProjectID: projectID,
		Title:     title,
		Level:     level,
		Status:    "pending",
		CreatedBy: createdBy,
		Version:   1,
		Stages:    stages,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if len(stages) > 0 {
		p.CurrentStage = stages[0].Type
	}
	return p
}

func defaultStages(level string) []Stage {
	if level == "L3" || level == "L4" {
		return []Stage{
			{Type: "clarify", Status: "pending"},
			{Type: "decompose", Status: "pending"},
			{Type: "impl", Status: "pending"},
			{Type: "test", Status: "pending"},
			{Type: "deploy", Status: "pending"},
			{Type: "verify", Status: "pending"},
		}
	}
	return []Stage{
		{Type: "clarify", Status: "pending"},
		{Type: "impl", Status: "pending"},
		{Type: "test", Status: "pending"},
		{Type: "deploy", Status: "pending"},
		{Type: "verify", Status: "pending"},
	}
}

// Transition validates and applies a state transition.
func (p *Pipeline) Transition(action string) error {
	next, ok := validTransitions[p.Status][action]
	if !ok {
		return fmt.Errorf("invalid transition: %q from %q", action, p.Status)
	}
	if action == "backtrack" {
		if p.BacktrackCount >= 3 {
			return fmt.Errorf("backtrack limit exceeded (3 max)")
		}
		p.BacktrackCount++
	}
	p.Status = next
	return nil
}

// AdvanceStage moves the pipeline to the next stage.
func (p *Pipeline) AdvanceStage() {
	currentIdx := p.currentStageIndex()
	if currentIdx < 0 || currentIdx >= len(p.Stages) {
		p.Status = "completed"
		return
	}
	p.Stages[currentIdx].Status = "passed"
	nextIdx := currentIdx + 1
	if nextIdx >= len(p.Stages) {
		p.Status = "completed"
		p.CurrentStage = ""
		return
	}
	p.Stages[nextIdx].Status = "running"
	p.CurrentStage = p.Stages[nextIdx].Type
	p.Status = "running"
}

func (p *Pipeline) currentStageIndex() int {
	for i, s := range p.Stages {
		if s.Type == p.CurrentStage {
			return i
		}
	}
	return -1
}

// CanBacktrack returns true if the pipeline can still backtrack.
func (p *Pipeline) CanBacktrack() bool {
	return p.BacktrackCount < 3
}

// BuildImplPrompt builds the Agent input prompt for the implementation stage.
func (p *Pipeline) BuildImplPrompt() string {
	return fmt.Sprintf("需求: %s\n模块: %+v", p.Title, p.Stages)
}

// NeedsGate uses PermissionMode to determine if gate approval is required.
func (p *Pipeline) NeedsGate() bool {
	mode := auth.SelectMode(p.Level, p.CurrentStage)
	return mode == auth.PermissionModeDefault
}

var validTransitions = map[string]map[string]string{
	"pending": {
		"start":  "running",
		"cancel": "cancelled",
	},
	"running": {
		"complete_stage": "awaiting_review",
		"pause":          "paused",
		"cancel":         "cancelled",
		"exceed_token":   "token_exceeded",
		"backtrack":      "dormant",
	},
	"paused": {
		"resume": "running",
		"cancel": "cancelled",
	},
	"awaiting_review": {
		"gate_approve": "running",
		"gate_reject":  "rejected",
	},
	"dormant": {
		"resume": "running",
	},
	"token_exceeded": {
		"resume": "running",
	},
}

// Fork creates a sub-pipeline inheriting parent context.
func (p *Pipeline) Fork(childID, title, createdBy string) *Pipeline {
	childLevel := p.Level
	if p.Level == "L1" {
		childLevel = "L2"
	} else if p.Level == "L2" {
		childLevel = "L3"
	} else {
		childLevel = "L3"
	}
	parentID := p.ID
	stages := defaultStages(childLevel)
	child := &Pipeline{
		ID:               childID,
		ProjectID:        p.ProjectID,
		ParentPipelineID: &parentID,
		Title:            title,
		Level:            childLevel,
		Status:           "pending",
		CurrentStage:     stages[0].Type,
		CreatedBy:        createdBy,
		Region:           p.Region,
		Config:           p.Config.Clone(),
		Stages:           stages,
		BacktrackCount:   0,
		Version:          1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	return child
}

// IsSubPipeline returns true if this is a sub-pipeline (has parent).
func (p *Pipeline) IsSubPipeline() bool {
	return p.ParentPipelineID != nil && *p.ParentPipelineID != ""
}
