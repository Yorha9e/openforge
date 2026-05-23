package port

import (
	"context"
	"time"

	"openforge/internal/pipeline/domain"
)

type PipelineRepository interface {
	Create(ctx context.Context, p *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error)
	UpdateStatus(ctx context.Context, id string, status string, version int) error
	IncrementBacktrack(ctx context.Context, id string) error
}

type GateRepository interface {
	CreateEvent(ctx context.Context, ev *domain.GateEvent) error
	GetLatestHash(ctx context.Context, pipelineID string) (string, error)
	ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error)
	ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error)
	Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error
	ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error
}

// TokenCostRow holds one aggregated data point for cost reporting.
type TokenCostRow struct {
	Date             string
	ProjectID        string
	Provider         string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
	EstimatedCost    float64
}

// ProjectBudget holds monthly budget config for a project.
type ProjectBudget struct {
	ProjectID      string
	MonthlyLimit   int64
	CurrentUsage   int64
	CostLimit      float64
	CurrentCost    float64
	ResetAt        time.Time
}

type TokenCostRepository interface {
	AggregateByDay(ctx context.Context, projectID string, days int) ([]TokenCostRow, error)
	AggregateByModel(ctx context.Context, projectID string, days int) ([]TokenCostRow, error)
	GetProjectBudget(ctx context.Context, projectID string) (*ProjectBudget, error)
	GetCurrentMonthUsage(ctx context.Context, projectID string) (int64, float64, error)
}
