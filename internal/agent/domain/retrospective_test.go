package domain

import (
	"context"
	"testing"
)

func TestPipelineRetrospective_Fields(t *testing.T) {
	r := &PipelineRetrospective{
		PipelineID:         "pipe-42",
		ProjectID:          "proj-A",
		DurationSeconds:    3600,
		ChatRounds:         25,
		TotalTokens:        120000,
		RejectionCount:     2,
		LessonsLearned:     []string{"use zod for validation", "add error boundaries"},
		ImprovementActions: []string{"set up zod schema template", "document error boundary pattern"},
		FailureCodes:       []string{"MODEL_HALLUCINATION"},
	}
	if r.PipelineID != "pipe-42" {
		t.Errorf("expected pipe-42, got %s", r.PipelineID)
	}
	if len(r.LessonsLearned) != 2 {
		t.Errorf("expected 2 lessons, got %d", len(r.LessonsLearned))
	}
	if len(r.FailureCodes) != 1 {
		t.Errorf("expected 1 failure code, got %d", len(r.FailureCodes))
	}
}

// memRetroStore is a minimal in-memory RetrospectiveStore for testing.
type memRetroStore struct {
	retros []*PipelineRetrospective
}

func newMemRetroStore() *memRetroStore {
	return &memRetroStore{}
}

func (s *memRetroStore) Create(_ context.Context, r *PipelineRetrospective) error {
	s.retros = append(s.retros, r)
	return nil
}

func (s *memRetroStore) ListByProject(_ context.Context, projectID string, limit int) ([]PipelineRetrospective, error) {
	var out []PipelineRetrospective
	for _, r := range s.retros {
		if r.ProjectID == projectID {
			out = append(out, *r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *memRetroStore) GetByPipeline(_ context.Context, pipelineID string) (*PipelineRetrospective, error) {
	for _, r := range s.retros {
		if r.PipelineID == pipelineID {
			return r, nil
		}
	}
	return nil, nil
}

var _ RetrospectiveStore = (*memRetroStore)(nil)

func TestRetrospectiveGenerator_Generate(t *testing.T) {
	ctx := context.Background()
	trajStore := NewMemTrajectoryStore()
	prefStore := NewMemPreferenceStore()
	retroStore := newMemRetroStore()

	// Seed a trajectory.
	_ = trajStore.Record(ctx, TrajectoryRecord{
		ProjectID:          "proj-gen",
		PipelineID:         "pipe-100",
		StageSequence:      []string{"plan", "code", "test"},
		TotalChatRounds:    8,
		TotalTokens:        45000,
		BacktrackCount:     2,
		RejectionCount:     1,
		FailureCodes:       []string{"MODEL_HALLUCINATION", "DEPENDENCY_CONFLICT"},
		SuccessfulPatterns: []string{"incremental-test"},
		ToolsUsed:          []string{"bash", "write_file", "grep"},
	})

	gen := NewRetrospectiveGenerator(retroStore, trajStore, prefStore)
	retro, err := gen.Generate(ctx, "proj-gen", "pipe-100")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if retro == nil {
		t.Fatal("expected non-nil retrospective")
	}
	if retro.PipelineID != "pipe-100" {
		t.Errorf("expected pipe-100, got %s", retro.PipelineID)
	}
	if retro.ProjectID != "proj-gen" {
		t.Errorf("expected proj-gen, got %s", retro.ProjectID)
	}

	// Lessons from failure codes
	if len(retro.LessonsLearned) == 0 {
		t.Errorf("expected lessons from failure codes, got empty")
	}

	// Improvement actions should be derived from known failure codes.
	if len(retro.ImprovementActions) == 0 {
		t.Errorf("expected improvement actions from failure codes, got empty")
	}

	// Successful patterns should appear in lessons.
	foundSuccess := false
	for _, l := range retro.LessonsLearned {
		if strContains(l, "incremental-test") {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Errorf("expected successful patterns in lessons, got %v", retro.LessonsLearned)
	}

	// Verify the retrospective was persisted.
	got, err := retroStore.GetByPipeline(ctx, "pipe-100")
	if err != nil {
		t.Fatalf("GetByPipeline: %v", err)
	}
	if got == nil {
		t.Fatal("retrospective should have been persisted")
	}
}

func TestRetrospectiveGenerator_Generate_NoTrajectory(t *testing.T) {
	ctx := context.Background()
	gen := NewRetrospectiveGenerator(newMemRetroStore(), NewMemTrajectoryStore(), NewMemPreferenceStore())

	_, err := gen.Generate(ctx, "proj-none", "pipe-nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent trajectory, got nil")
	}
	t.Logf("expected error: %v", err)
}

func TestRetrospectiveGenerator_Generate_CleanPipeline(t *testing.T) {
	ctx := context.Background()
	trajStore := NewMemTrajectoryStore()
	_ = trajStore.Record(ctx, TrajectoryRecord{
		ProjectID:          "proj-clean",
		PipelineID:         "pipe-clean",
		StageSequence:      []string{"plan", "code"},
		TotalChatRounds:    3,
		TotalTokens:        12000,
		SuccessfulPatterns: []string{"fast-build", "zero-errors"},
	})

	gen := NewRetrospectiveGenerator(newMemRetroStore(), trajStore, NewMemPreferenceStore())
	retro, err := gen.Generate(ctx, "proj-clean", "pipe-clean")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// No failure codes → no improvement actions.
	if len(retro.ImprovementActions) != 0 {
		t.Errorf("expected no improvement actions for clean pipeline, got %v", retro.ImprovementActions)
	}

	// Successful patterns should still appear as lessons.
	if len(retro.LessonsLearned) == 0 {
		t.Errorf("expected lessons from successful patterns, got empty")
	}
}

func TestRetrospectiveGenerator_CrossPipelineSummary(t *testing.T) {
	ctx := context.Background()
	trajStore := NewMemTrajectoryStore()
	prefStore := NewMemPreferenceStore()
	retroStore := newMemRetroStore()

	// Seed trajectories.
	for i := 0; i < 5; i++ {
		pipeID := "pipe-" + string(rune('a'+i))
		var codes []string
		if i == 0 {
			codes = []string{"MODEL_HALLUCINATION"}
		} else if i < 3 {
			codes = []string{"DEPENDENCY_CONFLICT"}
		}
		_ = trajStore.Record(ctx, TrajectoryRecord{
			ProjectID:      "proj-summary",
			PipelineID:     pipeID,
			FailureCodes:   codes,
		})
	}
	// Generate retrospectives for all 5 pipelines.
	gen := NewRetrospectiveGenerator(retroStore, trajStore, prefStore)
	for i := 0; i < 5; i++ {
		pipeID := "pipe-" + string(rune('a'+i))
		if _, err := gen.Generate(ctx, "proj-summary", pipeID); err != nil {
			t.Fatalf("Generate %s: %v", pipeID, err)
		}
	}

	summary, err := gen.CrossPipelineSummary(ctx, "proj-summary", 7)
	if err != nil {
		t.Fatalf("CrossPipelineSummary: %v", err)
	}

	if summary.ProjectID != "proj-summary" {
		t.Errorf("expected proj-summary, got %s", summary.ProjectID)
	}
	if summary.TotalPipelines != 5 {
		t.Errorf("expected 5 pipelines, got %d", summary.TotalPipelines)
	}

	// MODEL_HALLUCINATION appeared once, DEPENDENCY_CONFLICT appeared twice.
	if summary.RejectionFrequency["MODEL_HALLUCINATION"] != 1 {
		t.Errorf("expected 1 MODEL_HALLUCINATION, got %d", summary.RejectionFrequency["MODEL_HALLUCINATION"])
	}
	if summary.RejectionFrequency["DEPENDENCY_CONFLICT"] != 2 {
		t.Errorf("expected 2 DEPENDENCY_CONFLICT, got %d", summary.RejectionFrequency["DEPENDENCY_CONFLICT"])
	}

	// Since no code appeared 3+ times, suggested experiments should be empty.
	if len(summary.SuggestedExperiments) != 0 {
		t.Errorf("expected 0 suggested experiments (no code >=3), got %d", len(summary.SuggestedExperiments))
	}
}

func TestRetrospectiveGenerator_CrossPipelineSummary_WithSuggestedExperiments(t *testing.T) {
	ctx := context.Background()
	trajStore := NewMemTrajectoryStore()
	prefStore := NewMemPreferenceStore()
	retroStore := newMemRetroStore()

	// Create 3 pipelines with CONTEXT_OVERFLOW (triggers experiment suggestion).
	for i := 0; i < 3; i++ {
		pipeID := "pipe-" + string(rune('a'+i))
		_ = trajStore.Record(ctx, TrajectoryRecord{
			ProjectID:    "proj-suggest",
			PipelineID:   pipeID,
			FailureCodes: []string{"CONTEXT_OVERFLOW"},
		})
	}

	gen := NewRetrospectiveGenerator(retroStore, trajStore, prefStore)
	for i := 0; i < 3; i++ {
		pipeID := "pipe-" + string(rune('a'+i))
		if _, err := gen.Generate(ctx, "proj-suggest", pipeID); err != nil {
			t.Fatalf("Generate %s: %v", pipeID, err)
		}
	}

	summary, err := gen.CrossPipelineSummary(ctx, "proj-suggest", 7)
	if err != nil {
		t.Fatalf("CrossPipelineSummary: %v", err)
	}

	if len(summary.SuggestedExperiments) != 1 {
		t.Fatalf("expected 1 suggested experiment (CONTEXT_OVERFLOW x3), got %d", len(summary.SuggestedExperiments))
	}
	if !strContains(summary.SuggestedExperiments[0], "CONTEXT_OVERFLOW") {
		t.Errorf("expected suggestion mentioning CONTEXT_OVERFLOW, got %s", summary.SuggestedExperiments[0])
	}
}

// strContains is a helper for substring matching.
func strContains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
