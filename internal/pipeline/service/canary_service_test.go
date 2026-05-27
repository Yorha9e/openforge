package service

import (
	"testing"
	"time"

	"openforge/internal/policy/domain"
)

func TestCanaryService_EvaluateDelegatesToEngine(t *testing.T) {
	config := &domain.CanaryConfig{
		ID:         "canary-1",
		Target:     "feat-v2",
		Percentage: 100.0, // force apply
		Projects:   []string{"proj-1"},
		Duration:   24 * time.Hour,
		StartedAt:  time.Now(),
		Status:     "active",
		RollbackOn: domain.RollbackCondition{
			CodeRejectionIncrease: 0.1,
			MinSampleSize:         5,
		},
	}

	svc := NewCanaryService(config)
	results := svc.Evaluate("pipe-1", "proj-1", 0.05, 0.02, 10)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Apply {
		t.Fatal("expected canary to apply")
	}
}

func TestCanaryService_UpdateConfigsReplacesEngine(t *testing.T) {
	svc := NewCanaryService()
	if len(svc.Configs()) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(svc.Configs()))
	}

	config := &domain.CanaryConfig{
		ID:         "canary-1",
		Target:     "feat-v2",
		Status:     "active",
		StartedAt:  time.Now(),
		Duration:   time.Hour,
	}

	svc.Replace([]*domain.CanaryConfig{config})
	if len(svc.Configs()) != 1 {
		t.Fatalf("expected 1 config, got %d", len(svc.Configs()))
	}
}

func TestCanaryService_StatusReturnsConfigs(t *testing.T) {
	svc := NewCanaryService()
	if len(svc.Configs()) != 0 {
		t.Fatalf("expected empty configs list")
	}
}
