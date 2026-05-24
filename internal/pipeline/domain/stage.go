package domain

import "time"

type Stage struct {
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Summary     string     `json:"summary"`
	ArtifactRef string     `json:"artifact_ref"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
