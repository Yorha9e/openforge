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
	ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error)
	ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error)
	Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error
	ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error
}
