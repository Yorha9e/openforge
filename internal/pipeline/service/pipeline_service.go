package service

import (
	"context"

	"openforge/internal/pipeline/port"
)

type PipelineService struct {
	repo port.PipelineRepository
}

func NewPipelineService(repo port.PipelineRepository) *PipelineService {
	return &PipelineService{repo: repo}
}

func (s *PipelineService) Start(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("start"); err != nil {
		return err
	}
	p.Stages[0].Status = "running"
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) AdvanceStage(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Use §4.4 PermissionMode (replaces hardcoded L3/L4 check)
	if p.NeedsGate() {
		if err := p.Transition("complete_stage"); err != nil {
			return err
		}
		return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
	}

	// L1/L2 or auto/plan mode: advance directly (no gate)
	p.AdvanceStage()
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Pause(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("pause"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Resume(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("resume"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Cancel(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("cancel"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}
