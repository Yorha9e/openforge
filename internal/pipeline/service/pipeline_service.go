package service

import (
	"context"
	"fmt"
	"time"

	obsadapter "openforge/internal/observability/adapter"
	obsdomain "openforge/internal/observability/domain"
	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type PipelineService struct {
	repo    port.PipelineRepository
	metrics *obsadapter.PrometheusExporter
}

func NewPipelineService(repo port.PipelineRepository) *PipelineService {
	return &PipelineService{repo: repo}
}

// SetMetrics injects the Prometheus exporter used to record call-site
// metrics.  Safe to leave nil — every metric call is guarded.
func (s *PipelineService) SetMetrics(pe *obsadapter.PrometheusExporter) {
	s.metrics = pe
}

// recordIncr is a nil-safe wrapper around the exporter's Incr helper.
func (s *PipelineService) recordIncr(name obsdomain.MetricName) {
	if s.metrics == nil {
		return
	}
	s.metrics.Incr(string(name))
}

// recordObserve is a nil-safe wrapper around the exporter's Observe helper.
func (s *PipelineService) recordObserve(name obsdomain.MetricName, v float64) {
	if s.metrics == nil {
		return
	}
	s.metrics.Observe(string(name), v)
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
	if err := s.repo.UpdateStatus(ctx, id, p.Status, p.Version); err != nil {
		return err
	}
	// Path-D T1: pipeline created.
	s.recordIncr(obsdomain.MetricPipelineCreated)
	return nil
}

func (s *PipelineService) AdvanceStage(ctx context.Context, id string) error {
	started := time.Now()
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
	if err := s.repo.UpdateStatus(ctx, id, p.Status, p.Version); err != nil {
		return err
	}
	// Path-D T1: pipeline stage completed + duration observed.
	s.recordIncr(obsdomain.MetricPipelineCompleted)
	s.recordObserve(obsdomain.MetricPipelineDuration, time.Since(started).Seconds())
	return nil
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

func (s *PipelineService) Fork(ctx context.Context, parentID, title, createdBy string) (*domain.Pipeline, error) {
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	childID := "pipe-" + fmt.Sprintf("%d", time.Now().UnixNano())
	child := parent.Fork(childID, title, createdBy)
	if err := s.repo.Create(ctx, child); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, childID)
}
