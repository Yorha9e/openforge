package domain

import (
	"testing"
	"time"
)

func TestCanary_ShouldApply_Distribution(t *testing.T) {
	c := &CanaryConfig{
		ID: "canary-1", Target: "v2", Percentage: 20,
		Projects: []string{"proj-a"},
		Status: "active", StartedAt: time.Now(), Duration: time.Hour,
	}
	engine := NewCanaryEngine([]*CanaryConfig{c})

	applied := 0
	for i := 0; i < 1000; i++ {
		pid := "pipe-" + string(rune(i))
		results := engine.Evaluate(pid, "proj-a", 0, 0, 0)
		if len(results) > 0 && results[0].Apply {
			applied++
		}
	}

	ratio := float64(applied) / 1000.0
	if ratio < 0.10 || ratio > 0.30 {
		t.Errorf("expected ~20%% canary ratio, got %.1f%%", ratio*100)
	}
}

func TestCanary_ProjectMatching(t *testing.T) {
	c := &CanaryConfig{
		ID: "canary-1", Target: "v2", Percentage: 100,
		Projects: []string{"proj-a", "proj-b"},
		Status: "active", StartedAt: time.Now(), Duration: time.Hour,
	}
	engine := NewCanaryEngine([]*CanaryConfig{c})

	t.Run("matching project", func(t *testing.T) {
		results := engine.Evaluate("pipe-1", "proj-a", 0, 0, 0)
		if len(results) != 1 || !results[0].Apply {
			t.Error("expected canary to apply for proj-a")
		}
	})

	t.Run("non-matching project", func(t *testing.T) {
		results := engine.Evaluate("pipe-1", "proj-c", 0, 0, 0)
		if len(results) != 0 {
			t.Error("expected no results for non-matching project")
		}
	})
}

func TestCanary_InactiveDoesNotApply(t *testing.T) {
	c := &CanaryConfig{
		ID: "canary-1", Target: "v2", Percentage: 100,
		Projects: []string{"proj-a"},
		Status: "completed", StartedAt: time.Now(), Duration: time.Hour,
	}
	engine := NewCanaryEngine([]*CanaryConfig{c})

	results := engine.Evaluate("pipe-1", "proj-a", 0, 0, 0)
	if len(results) != 0 {
		t.Error("completed canary should not produce results")
	}
}

func TestCanary_ExpiredDoesNotApply(t *testing.T) {
	c := &CanaryConfig{
		ID: "canary-1", Target: "v2", Percentage: 100,
		Projects: []string{"proj-a"},
		Status: "active", StartedAt: time.Now().Add(-2 * time.Hour), Duration: time.Hour,
	}
	engine := NewCanaryEngine([]*CanaryConfig{c})

	results := engine.Evaluate("pipe-1", "proj-a", 0, 0, 0)
	if len(results) != 0 {
		t.Error("expired canary should not produce results")
	}
}

func TestCanary_RollbackDetection(t *testing.T) {
	c := &CanaryConfig{
		ID: "canary-1", Target: "v2", Percentage: 100,
		Projects: []string{"proj-a"},
		Status: "active", StartedAt: time.Now(), Duration: time.Hour,
		RollbackOn: RollbackCondition{
			CodeRejectionIncrease: 0.15,
			MinSampleSize:         10,
		},
	}
	engine := NewCanaryEngine([]*CanaryConfig{c})

	t.Run("rejection increase triggers rollback", func(t *testing.T) {
		results := engine.Evaluate("pipe-1", "proj-a", 0.50, 0.20, 20)
		if len(results) != 1 {
			t.Fatal("expected one result")
		}
		if !results[0].Rollback {
			t.Error("expected rollback when rejection rate increased 30%")
		}
		if results[0].Reason == "" {
			t.Error("expected rollback reason")
		}
	})

	t.Run("small increase does not trigger rollback", func(t *testing.T) {
		results := engine.Evaluate("pipe-2", "proj-a", 0.25, 0.20, 20)
		if len(results) != 1 {
			t.Fatal("expected one result")
		}
		if results[0].Rollback {
			t.Error("expected no rollback when increase is within threshold")
		}
	})

	t.Run("below min sample skips rollback check", func(t *testing.T) {
		results := engine.Evaluate("pipe-3", "proj-a", 0.50, 0.20, 5)
		if len(results) != 1 {
			t.Fatal("expected one result")
		}
		if results[0].Rollback {
			t.Error("expected no rollback when below min sample size")
		}
	})
}
