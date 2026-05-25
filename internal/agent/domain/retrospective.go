package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// PipelineRetrospective captures post-pipeline analysis (§3.12).
type PipelineRetrospective struct {
	ID                 string
	PipelineID         string
	ProjectID          string
	DurationSeconds    int
	ChatRounds         int
	TotalTokens        int64
	RejectionCount     int
	BacktrackCount     int
	FailureCodes       []string
	LessonsLearned     []string
	ImprovementActions []string
	KnowledgeUpdates   []string
	CreatedAt          time.Time
}

// RetrospectiveStore persists pipeline retrospectives.
type RetrospectiveStore interface {
	Create(ctx context.Context, r *PipelineRetrospective) error
	ListByProject(ctx context.Context, projectID string, limit int) ([]PipelineRetrospective, error)
	GetByPipeline(ctx context.Context, pipelineID string) (*PipelineRetrospective, error)
}

// RetrospectiveSummary is the cross-pipeline weekly analysis output.
type RetrospectiveSummary struct {
	ProjectID            string
	PeriodStart          time.Time
	PeriodEnd            time.Time
	TotalPipelines       int
	RejectionFrequency   map[string]int
	TopLessons           []string
	SuggestedExperiments []string
	PromotedKnowledge    []string
	DiscardedKnowledge   []string
}

// RetrospectiveGenerator creates retrospectives from trajectory + preference data.
type RetrospectiveGenerator struct {
	retroStore RetrospectiveStore
	trajStore  TrajectoryStore
	prefStore  PreferenceStore
}

// NewRetrospectiveGenerator creates a new RetrospectiveGenerator.
func NewRetrospectiveGenerator(
	retroStore RetrospectiveStore,
	trajStore TrajectoryStore,
	prefStore PreferenceStore,
) *RetrospectiveGenerator {
	return &RetrospectiveGenerator{
		retroStore: retroStore,
		trajStore:  trajStore,
		prefStore:  prefStore,
	}
}

// Generate creates a retrospective for a completed pipeline.
func (g *RetrospectiveGenerator) Generate(ctx context.Context, projectID, pipelineID string) (*PipelineRetrospective, error) {
	traj, err := g.trajStore.GetByPipeline(ctx, pipelineID)
	if err != nil || traj == nil {
		return nil, fmt.Errorf("trajectory %s: %w", pipelineID, err)
	}

	var lessons, actions []string
	for _, code := range traj.FailureCodes {
		lessons = append(lessons, fmt.Sprintf("Failure %s: future pipelines should verify before proceeding", code))
		switch code {
		case "MODEL_HALLUCINATION":
			actions = append(actions, "Add API existence verification before code generation")
		case "DEPENDENCY_CONFLICT":
			actions = append(actions, "Lock dependency versions in sandbox pre-check")
		case "CONTEXT_OVERFLOW":
			actions = append(actions, "Reduce scope per pipeline; split large features")
		}
	}
	if len(traj.SuccessfulPatterns) > 0 {
		lessons = append(lessons, "Successful patterns: "+strings.Join(traj.SuccessfulPatterns, ", "))
	}

	var knowledgeIDs []string
	prefs, _ := g.prefStore.ListByProject(ctx, projectID)
	for _, p := range prefs {
		if p.LastActivated != "" {
			knowledgeIDs = append(knowledgeIDs, fmt.Sprintf("%s=%s", p.Key, p.Value))
		}
	}

	r := &PipelineRetrospective{
		PipelineID:         pipelineID,
		ProjectID:          projectID,
		ChatRounds:         traj.TotalChatRounds,
		TotalTokens:        traj.TotalTokens,
		RejectionCount:     traj.RejectionCount,
		BacktrackCount:     traj.BacktrackCount,
		FailureCodes:       traj.FailureCodes,
		LessonsLearned:     lessons,
		ImprovementActions: actions,
		KnowledgeUpdates:   knowledgeIDs,
		CreatedAt:          time.Now(),
	}
	if err := g.retroStore.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// CrossPipelineSummary generates a weekly cross-pipeline analysis (§3.12).
func (g *RetrospectiveGenerator) CrossPipelineSummary(ctx context.Context, projectID string, days int) (*RetrospectiveSummary, error) {
	retros, err := g.retroStore.ListByProject(ctx, projectID, days)
	if err != nil {
		return nil, err
	}

	summary := &RetrospectiveSummary{
		ProjectID:          projectID,
		TotalPipelines:     len(retros),
		RejectionFrequency: make(map[string]int),
	}

	for _, r := range retros {
		for _, code := range r.FailureCodes {
			summary.RejectionFrequency[code]++
		}
		if len(r.LessonsLearned) > 0 {
			summary.TopLessons = append(summary.TopLessons, r.LessonsLearned[0])
		}
	}

	for code, count := range summary.RejectionFrequency {
		if count >= 3 {
			summary.SuggestedExperiments = append(summary.SuggestedExperiments,
				fmt.Sprintf("AB test for %s (occurred %d times)", code, count))
		}
	}

	return summary, nil
}
