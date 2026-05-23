package service

import (
	"context"
	"testing"

	"openforge/internal/pipeline/port"
)

type stubTokenCostRepo struct {
	daily  []port.TokenCostRow
	models []port.TokenCostRow
	budget *port.ProjectBudget
}

func (s *stubTokenCostRepo) AggregateByDay(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	return s.daily, nil
}
func (s *stubTokenCostRepo) AggregateByModel(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	return s.models, nil
}
func (s *stubTokenCostRepo) GetProjectBudget(ctx context.Context, projectID string) (*port.ProjectBudget, error) {
	return s.budget, nil
}
func (s *stubTokenCostRepo) GetCurrentMonthUsage(ctx context.Context, projectID string) (int64, float64, error) {
	return 1000, 5.0, nil
}

func TestTokenCostService_DailyUsage(t *testing.T) {
	repo := &stubTokenCostRepo{daily: []port.TokenCostRow{{Date: "2026-05-23", PromptTokens: 100}}}
	svc := NewTokenCostService(repo)
	rows, err := svc.DailyUsage(context.Background(), "proj-1", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(rows))
	}
}

func TestTokenCostService_Budget(t *testing.T) {
	repo := &stubTokenCostRepo{budget: &port.ProjectBudget{ProjectID: "proj-1", MonthlyLimit: 50000}}
	svc := NewTokenCostService(repo)
	b, err := svc.Budget(context.Background(), "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.MonthlyLimit != 50000 {
		t.Errorf("MonthlyLimit = %d, want 50000", b.MonthlyLimit)
	}
	if b.CurrentUsage != 1000 {
		t.Errorf("CurrentUsage = %d, want 1000", b.CurrentUsage)
	}
}
