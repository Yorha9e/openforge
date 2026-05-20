package domain

import "time"

type Pipeline struct {
	ID             string
	ProjectID      string
	Title          string
	Level          string
	Status         string
	CurrentStage   string
	CreatedBy      string
	BacktrackCount int
	Stages         []Stage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewPipeline(id, projectID, title, createdBy string, files, modules int) *Pipeline {
	level := ClassifyComplexity(files, modules)
	return &Pipeline{
		ID:        id,
		ProjectID: projectID,
		Title:     title,
		Level:     level,
		Status:    "pending",
		CreatedBy: createdBy,
		Stages:    defaultStages(level),
	}
}

func defaultStages(level string) []Stage {
	stages := []Stage{
		{Type: "clarify", Status: "pending"},
		{Type: "impl", Status: "pending"},
		{Type: "test", Status: "pending"},
		{Type: "deploy", Status: "pending"},
		{Type: "verify", Status: "pending"},
	}
	if level == "L3" || level == "L4" {
		stages = append([]Stage{{Type: "decompose", Status: "pending"}}, stages...)
	}
	return stages
}
