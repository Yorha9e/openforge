package domain

import "time"

type Stage struct {
	Type        string
	Status      string
	Summary     string
	ArtifactRef string
	StartedAt   *time.Time
	CompletedAt *time.Time
}
