package domain

import (
	"context"
	"strings"
	"testing"
)

// TestLearningLayers_L1ExtractsFunctionSignatures verifies that ExtractL1
// picks up Go-style function signatures from a diff and returns them in order.
func TestLearningLayers_L1ExtractsFunctionSignatures(t *testing.T) {
	layers := NewLearningLayers(nil, nil, nil, NewInMemoryEmbeddingIndex())

	diff := `diff --git a/foo.go b/foo.go
@@
+func Hello() {}
+func World(x int) {}
`
	insights := layers.ExtractL1("p-1", diff)
	if len(insights) != 2 {
		t.Fatalf("expected 2 L1 insights, got %d", len(insights))
	}
	if insights[0].FuncName != "Hello" {
		t.Errorf("expected first FuncName=Hello, got %q", insights[0].FuncName)
	}
	if insights[1].FuncName != "World" {
		t.Errorf("expected second FuncName=World, got %q", insights[1].FuncName)
	}
	if insights[0].PipelineID != "p-1" {
		t.Errorf("expected PipelineID=p-1, got %q", insights[0].PipelineID)
	}
}

// TestLearningLayers_L2Diff verifies the simple line-count diff statistic.
func TestLearningLayers_L2Diff(t *testing.T) {
	layers := NewLearningLayers(nil, nil, nil, NewInMemoryEmbeddingIndex())

	oldDiff := "a\nb\nc\n"
	newDiff := "a\nb\nd\n"

	insight := layers.ExtractL2("p-1", oldDiff, newDiff)
	if insight.NewLines != 3 {
		t.Errorf("expected NewLines=3, got %d", insight.NewLines)
	}
	if insight.OldLines != 3 {
		t.Errorf("expected OldLines=3, got %d", insight.OldLines)
	}
	if insight.CommonLines < 0 {
		t.Errorf("expected CommonLines >= 0, got %d", insight.CommonLines)
	}
}

// TestLearningLayers_L3Trajectory verifies the trajectory tool frequency
// and step count are extracted from a mock store.
func TestLearningLayers_L3Trajectory(t *testing.T) {
	trajStore := NewMemTrajectoryStore()
	_ = trajStore.Record(context.Background(), TrajectoryRecord{
		ProjectID:     "proj-1",
		PipelineID:    "p-1",
		StageSequence: []string{"plan", "implement", "test"},
		ToolsUsed:     []string{"go", "go", "go", "test"},
	})

	layers := NewLearningLayers(trajStore, nil, nil, NewInMemoryEmbeddingIndex())
	insight := layers.ExtractL3("p-1")

	if insight.ToolFrequency["go"] != 3 {
		t.Errorf("expected go frequency 3, got %d", insight.ToolFrequency["go"])
	}
	if insight.ToolFrequency["test"] != 1 {
		t.Errorf("expected test frequency 1, got %d", insight.ToolFrequency["test"])
	}
	if insight.StepCount != 3 {
		t.Errorf("expected StepCount=3, got %d", insight.StepCount)
	}
}

// TestLearningLayers_L4Embedding verifies the embedding index layer returns
// non-empty top hits when the index has been pre-seeded.
func TestLearningLayers_L4Embedding(t *testing.T) {
	idx := NewInMemoryEmbeddingIndex()
	idx.Add("snap-1", "go is great")
	idx.Add("snap-2", "python is great")

	layers := NewLearningLayers(nil, nil, nil, idx)
	insight := layers.ExtractL4("go")

	if len(insight.TopHits) == 0 {
		t.Fatalf("expected non-empty TopHits, got 0")
	}
	if insight.TopHits[0].ID != "snap-1" {
		t.Errorf("expected top hit ID snap-1, got %q", insight.TopHits[0].ID)
	}
	// Sanity-check score is positive.
	if insight.TopHits[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", insight.TopHits[0].Score)
	}
	// Defensive guard: also confirm the input text is preserved.
	if !strings.Contains(insight.Query, "go") {
		t.Errorf("expected query to contain 'go', got %q", insight.Query)
	}
}
