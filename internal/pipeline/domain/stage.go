package domain

import "time"

// Stage represents a single stage within a pipeline execution.
type Stage struct {
	ID                 string     `json:"id"`
	PipelineID         string     `json:"pipeline_id"`
	Type               string     `json:"type"` // clarify, decompose, impl, test, deploy, verify
	Status             string     `json:"status"` // pending, running, awaiting_gate, passed, failed, skipped
	RequirementSummary string     `json:"requirement_summary,omitempty"`
	Constraints        []string   `json:"constraints,omitempty"`
	PreferenceProfile  string     `json:"preference_profile,omitempty"`
	ModuleIndexSubset  string     `json:"module_index_subset,omitempty"`
	Summary            string     `json:"summary,omitempty"`
	ArtifactRef        string     `json:"artifact_ref"`
	ArtifactHash       string     `json:"artifact_hash,omitempty"`
	SchemaVersion      int        `json:"schema_version"`
	Version            int        `json:"version"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

// ValidStages lists the allowed stage types.
var ValidStages = []string{"clarify", "decompose", "impl", "test", "deploy", "verify"}

// ValidStageStatuses lists the allowed stage statuses.
var ValidStageStatuses = []string{"pending", "running", "awaiting_gate", "passed", "failed", "skipped"}

// IsValidStage checks if the given stage type is valid.
func IsValidStage(stage string) bool {
	for _, s := range ValidStages {
		if s == stage {
			return true
		}
	}
	return false
}

// IsValidStageStatus checks if the given status is valid.
func IsValidStageStatus(status string) bool {
	for _, s := range ValidStageStatuses {
		if s == status {
			return true
		}
	}
	return false
}
