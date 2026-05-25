package domain

import (
	"context"
	"testing"
)

func TestABSimpleTTest_EqualMeans(t *testing.T) {
	control := []float64{10, 11, 9, 10, 10}
	treatment := []float64{10, 11, 9, 10, 10}
	p, effect := SimpleTTest(control, treatment)
	if effect != 0 {
		t.Errorf("expected zero effect, got %f", effect)
	}
	if p < 0.5 {
		t.Errorf("expected high p-value for identical means, got %f", p)
	}
}

func TestABSimpleTTest_PositiveEffect(t *testing.T) {
	control := []float64{10, 10, 10, 10, 10}
	treatment := []float64{15, 15, 15, 15, 15}
	p, effect := SimpleTTest(control, treatment)
	if effect <= 0 {
		t.Errorf("expected positive effect, got %f", effect)
	}
	if p > 0.05 {
		t.Errorf("expected significant p-value, got %f", p)
	}
}

func TestABSimpleTTest_InsufficientData(t *testing.T) {
	p, effect := SimpleTTest([]float64{1}, []float64{2})
	if p != 1.0 || effect != 0 {
		t.Errorf("expected (1, 0) for insufficient data, got (%f, %f)", p, effect)
	}
}

func TestABDetermineVerdict_Promoted(t *testing.T) {
	v := DetermineVerdict(0.01, 0.5)
	if v != "promoted" {
		t.Errorf("expected promoted, got %s", v)
	}
}

func TestABDetermineVerdict_Harmful(t *testing.T) {
	v := DetermineVerdict(0.01, -0.5)
	if v != "harmful" {
		t.Errorf("expected harmful, got %s", v)
	}
}

func TestABDetermineVerdict_Invalid(t *testing.T) {
	tests := []struct {
		pValue     float64
		effectSize float64
	}{
		{0.01, 0},       // significant, zero effect
		{0.5, -0.3},     // not significant, negative effect
		{0.5, 0},        // not significant, zero effect
	}
	for _, tt := range tests {
		v := DetermineVerdict(tt.pValue, tt.effectSize)
		if v != "invalid" {
			t.Errorf("for p=%f effect=%f: expected invalid, got %s", tt.pValue, tt.effectSize, v)
		}
	}
}

func TestABDetermineVerdict_Inconclusive(t *testing.T) {
	v := DetermineVerdict(0.5, 0.3)
	if v != "" {
		t.Errorf("expected empty verdict, got %s", v)
	}
}

func TestABMemStore_CreateAndGet(t *testing.T) {
	store := NewMemExperimentStore()
	exp := &ABExperiment{ID: "exp-1", KnowledgeID: "k-1", Status: "running"}
	if err := store.Create(context.Background(), exp); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := store.Get(context.Background(), "exp-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "exp-1" || got.KnowledgeID != "k-1" {
		t.Errorf("unexpected experiment: %+v", got)
	}
}

func TestABMemStore_Assign(t *testing.T) {
	store := NewMemExperimentStore()
	a := &ABExperimentAssignment{ID: "a-1", ExperimentID: "exp-1", PipelineID: "pipe-1", Cohort: "A"}
	if err := store.Assign(context.Background(), a); err != nil {
		t.Fatalf("Assign: %v", err)
	}
}

func TestABMemStore_Complete(t *testing.T) {
	store := NewMemExperimentStore()
	store.Create(context.Background(), &ABExperiment{ID: "exp-1", KnowledgeID: "k-1", Status: "running"})
	if err := store.Complete(context.Background(), "exp-1", "promoted", 0.01, 0.5); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, _ := store.Get(context.Background(), "exp-1")
	if got.Status != "completed" || got.Verdict != "promoted" || got.PValue != 0.01 || got.EffectSize != 0.5 {
		t.Errorf("unexpected completed experiment: %+v", got)
	}
}

func TestABMemStore_ListActive(t *testing.T) {
	store := NewMemExperimentStore()
	store.Create(context.Background(), &ABExperiment{ID: "exp-1", Status: "running"})
	store.Create(context.Background(), &ABExperiment{ID: "exp-2", Status: "completed"})
	store.Create(context.Background(), &ABExperiment{ID: "exp-3", Status: "running"})
	active, err := store.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active experiments, got %d", len(active))
	}
}
