package service

import (
	"context"

	"openforge/internal/pipeline/port"
)

type TokenCostService struct {
	repo port.TokenCostRepository
}

func NewTokenCostService(repo port.TokenCostRepository) *TokenCostService {
	return &TokenCostService{repo: repo}
}

func (s *TokenCostService) DailyUsage(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	return s.repo.AggregateByDay(ctx, projectID, days)
}

func (s *TokenCostService) ModelBreakdown(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	return s.repo.AggregateByModel(ctx, projectID, days)
}

func (s *TokenCostService) Budget(ctx context.Context, projectID string) (*port.ProjectBudget, error) {
	b, err := s.repo.GetProjectBudget(ctx, projectID)
	if err != nil {
		return nil, err
	}
	b.CurrentUsage, b.CurrentCost, _ = s.repo.GetCurrentMonthUsage(ctx, projectID)
	return b, nil
}
