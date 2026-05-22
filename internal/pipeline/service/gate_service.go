package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type GateService struct {
	gateRepo port.GateRepository
	pipeRepo port.PipelineRepository
}

func NewGateService(gateRepo port.GateRepository, pipeRepo port.PipelineRepository) *GateService {
	return &GateService{
		gateRepo: gateRepo,
		pipeRepo: pipeRepo,
	}
}

func (s *GateService) Approve(ctx context.Context, pipelineID, stage, actor string, checklist domain.GateChecklist, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s|%s|%s|approve", pipelineID, stage, actor)
	ev := &domain.GateEvent{
		PipelineID:      pipelineID,
		Stage:           stage,
		Event:           "approved",
		Actor:           actor,
		Decision:        "approve",
		SummaryFeedback: summary,
		Checklist:       checklist,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
		PrevHash:        "genesis",
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}

	if err := p.Transition("gate_approve"); err != nil {
		return err
	}
	p.AdvanceStage()
	return s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version)
}

func (s *GateService) Reject(ctx context.Context, pipelineID, stage, actor string, comments []domain.LineComment, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	ev := &domain.GateEvent{
		PipelineID:      pipelineID,
		Stage:           stage,
		Event:           "rejected",
		Actor:           actor,
		Decision:        "reject",
		LineComments:    comments,
		SummaryFeedback: summary,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|reject", pipelineID, stage, actor)))),
		PrevHash:        "genesis",
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}

	if err := p.Transition("gate_reject"); err != nil {
		return err
	}
	return s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version)
}

func (s *GateService) Claim(ctx context.Context, pipelineID, stage, actor string) error {
	return s.gateRepo.Claim(ctx, pipelineID, stage, actor, 30*time.Minute)
}

func (s *GateService) Release(ctx context.Context, pipelineID, stage, actor string) error {
	return s.gateRepo.ReleaseClaim(ctx, pipelineID, stage, actor)
}

func (s *GateService) ListPending(ctx context.Context) ([]*domain.GateEvent, error) {
	return s.gateRepo.ListPending(ctx, "")
}
