package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	obsadapter "openforge/internal/observability/adapter"
	obsdomain "openforge/internal/observability/domain"
	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type GateService struct {
	gateRepo port.GateRepository
	pipeRepo port.PipelineRepository
	hooks    domain.HookChain
	metrics  *obsadapter.PrometheusExporter
}

func NewGateService(gateRepo port.GateRepository, pipeRepo port.PipelineRepository, hooks ...domain.GateHook) *GateService {
	return &GateService{
		gateRepo: gateRepo,
		pipeRepo: pipeRepo,
		hooks:    hooks,
	}
}

// SetMetrics injects the Prometheus exporter used to record call-site
// metrics.  Safe to leave nil — every metric call is guarded.
func (s *GateService) SetMetrics(pe *obsadapter.PrometheusExporter) {
	s.metrics = pe
}

// recordIncr is a nil-safe wrapper around the exporter's Incr helper.
func (s *GateService) recordIncr(name obsdomain.MetricName) {
	if s.metrics == nil {
		return
	}
	s.metrics.Incr(string(name))
}

func (s *GateService) Approve(ctx context.Context, pipelineID, stage, actor string, checklist domain.GateChecklist, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	prevHash, err := s.gateRepo.GetLatestHash(ctx, pipelineID)
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
		PrevHash:        prevHash,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content))),
	}

	if err := s.hooks.RunPreApprove(ctx, ev); err != nil {
		return err
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}
	s.hooks.RunPostApprove(ctx, ev)

	if err := p.Transition("gate_approve"); err != nil {
		return err
	}
	p.AdvanceStage()
	if err := s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version); err != nil {
		return err
	}
	// Path-D T1: gate approved.
	s.recordIncr(obsdomain.MetricGateApproveTotal)
	return nil
}

func (s *GateService) Reject(ctx context.Context, pipelineID, stage, actor string, comments []domain.LineComment, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	prevHash, err := s.gateRepo.GetLatestHash(ctx, pipelineID)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s|%s|%s|reject", pipelineID, stage, actor)
	ev := &domain.GateEvent{
		PipelineID:      pipelineID,
		Stage:           stage,
		Event:           "rejected",
		Actor:           actor,
		Decision:        "reject",
		LineComments:    comments,
		SummaryFeedback: summary,
		PrevHash:        prevHash,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content))),
	}

	if err := s.hooks.RunPreReject(ctx, ev); err != nil {
		return err
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}
	s.hooks.RunPostReject(ctx, ev)

	if err := p.Transition("gate_reject"); err != nil {
		return err
	}
	if err := s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version); err != nil {
		return err
	}
	// Path-D T1: gate rejected.
	s.recordIncr(obsdomain.MetricGateRejectTotal)
	return nil
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
