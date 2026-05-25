package domain

import (
	"context"
	"testing"
)

func TestMemTrajectoryStore_RecordAndList(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-1",
		ToolsUsed: []string{"bash", "write_file"},
		SkillsMatched: []string{"react-pattern", "testing"},
		FailureCodes: []string{"MODEL_HALLUCINATION"},
	})

	list, err := store.ListByProject(ctx, "proj-A")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
}

func TestMemTrajectoryStore_SuccessfulTools(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p1", ToolsUsed: []string{"bash", "grep"}})
	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p2", ToolsUsed: []string{"bash", "write_file"}, FailureCodes: []string{"ERR"}})
	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p3", ToolsUsed: []string{"bash"}})

	tools, err := store.SuccessfulTools(ctx, "proj-A", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if tools[0] != "bash" {
		t.Errorf("most successful tool should be bash, got %v", tools)
	}
}

func TestMemTrajectoryStore_SimilarPatterns(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-1",
		FailureCodes: []string{"MODEL_HALLUCINATION", "CONTEXT_OVERFLOW"},
	})
	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-2",
		FailureCodes: []string{"DEPENDENCY_CONFLICT"},
	})

	matches, _ := store.SimilarPatterns(ctx, "proj-A", []string{"MODEL_HALLUCINATION"}, 10)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}
