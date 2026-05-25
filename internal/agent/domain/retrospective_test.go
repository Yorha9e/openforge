package domain

import (
	"testing"
	"time"
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
		CreatedAt:          time.Now(),
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
