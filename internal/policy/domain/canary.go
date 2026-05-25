package domain

import (
	"math/rand"
	"time"
)

// RollbackCondition defines the criteria for auto-rolling back a canary.
type RollbackCondition struct {
	CodeRejectionIncrease float64 // e.g. 0.15 = 15% increase triggers rollback
	MinSampleSize         int     // minimum pipelines before evaluating
}

// CanaryConfig defines a canary rollout for pipeline changes.
type CanaryConfig struct {
	ID         string
	Target     string            // e.g. "code-generate.v2"
	Percentage float64           // 0-100
	Projects   []string          // matched project IDs
	Duration   time.Duration     // evaluation window
	StartedAt  time.Time
	Status     string            // "active" | "completed" | "rolled_back"
	RollbackOn RollbackCondition
}

// IsActive returns true if the canary is still in its evaluation window.
func (c *CanaryConfig) IsActive() bool {
	return c.Status == "active" && time.Since(c.StartedAt) < c.Duration
}

// CanaryEngine evaluates canary applicability and rollback conditions.
type CanaryEngine struct {
	canaries map[string]*CanaryConfig
	rng      *rand.Rand
}

// NewCanaryEngine creates a CanaryEngine with the given canary configs.
func NewCanaryEngine(canaries []*CanaryConfig) *CanaryEngine {
	m := make(map[string]*CanaryConfig, len(canaries))
	for _, c := range canaries {
		m[c.ID] = c
	}
	return &CanaryEngine{
		canaries: m,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// EvaluateResult is the outcome of a canary evaluation for a pipeline.
type EvaluateResult struct {
	CanaryID  string
	Apply     bool // whether the canary version should be used
	Rollback  bool // whether auto-rollback is triggered
	Reason    string
}

// Evaluate checks whether a pipeline should use the canary version.
// It considers percentage routing, project matching, and auto-rollback.
func (e *CanaryEngine) Evaluate(pipelineID, projectID string, currentRejectionRate, baselineRejectionRate float64, sampleSize int) []EvaluateResult {
	var results []EvaluateResult
	for _, c := range e.canaries {
		if !c.IsActive() {
			continue
		}

		// Project matching: skip if this project is not in the canary scope.
		if !e.matchesProject(c, projectID) {
			continue
		}

		// Percentage routing: deterministic hash-based assignment.
		apply := e.shouldApply(c, pipelineID)

		// Auto-rollback check.
		rollback := false
		reason := ""
		if apply && sampleSize >= c.RollbackOn.MinSampleSize {
			increase := currentRejectionRate - baselineRejectionRate
			if increase > c.RollbackOn.CodeRejectionIncrease {
				rollback = true
				reason = "code rejection rate increased beyond threshold"
			}
		}

		results = append(results, EvaluateResult{
			CanaryID: c.ID,
			Apply:    apply,
			Rollback: rollback,
			Reason:   reason,
		})
	}
	return results
}

// matchesProject returns true if projectID is in the canary's allowed projects.
func (e *CanaryEngine) matchesProject(c *CanaryConfig, projectID string) bool {
	for _, p := range c.Projects {
		if p == projectID {
			return true
		}
	}
	return false
}

// shouldApply deterministically assigns the pipeline to the canary group
// based on a hash of the pipeline ID and the configured percentage.
func (e *CanaryEngine) shouldApply(c *CanaryConfig, pipelineID string) bool {
	hash := 0
	for _, ch := range pipelineID {
		hash = hash*31 + int(ch)
	}
	return float64(hash%100) < c.Percentage
}
