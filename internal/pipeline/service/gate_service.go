package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

// GateAuditEvent is the minimal audit record written when a TOCTOU tampering
// attempt is blocked at gate-approval time. We keep this as a small local
// interface (rather than depending on policy/adapter.AuditLogger) to avoid
// introducing a reverse dependency from pipeline/service into policy/adapter.
type GateAuditEvent struct {
	EventID string
	Actor   string
	Action  string
	Details string
}

// GateAuditor is the audit sink used by GateService to record gate-level
// security events (e.g. TOCTOU blocking). A nil auditor is treated as a
// no-op sink so existing call sites (bootstrap) keep working without
// injection.
type GateAuditor interface {
	Log(ctx context.Context, ev GateAuditEvent)
}

// noopGateAuditor is the default auditor used when none is wired in.
type noopGateAuditor struct{}

func (noopGateAuditor) Log(_ context.Context, _ GateAuditEvent) {}

// GatePipelineAdvancer is the downstream-advance hook invoked by
// ApproveAndAdvance after a successful artifact-hash verification. It is
// defined as a small interface so the test can inject a fake without pulling
// in the concrete *PipelineService (which would in turn require this package
// to construct one with a repository).
type GatePipelineAdvancer interface {
	AdvanceAfterGate(ctx context.Context, pipelineID, stage string) error
}

type GateService struct {
	gateRepo port.GateRepository
	pipeRepo port.PipelineRepository
	hooks    domain.HookChain
	auditor  GateAuditor
	advancer GatePipelineAdvancer
}

// NewGateService constructs a GateService. The optional opts allow callers
// to inject an audit sink and a downstream advancer without forcing a
// signature change for existing bootstrap call sites. Pass a nil auditor or
// advancer to fall back to safe no-op behavior.
func NewGateService(gateRepo port.GateRepository, pipeRepo port.PipelineRepository, hooks ...domain.GateHook) *GateService {
	return &GateService{
		gateRepo: gateRepo,
		pipeRepo: pipeRepo,
		hooks:    hooks,
		auditor:  noopGateAuditor{},
		advancer: nil,
	}
}

// WithAuditor returns a new GateService with the given auditor attached.
// Provided as a fluent builder to avoid breaking the existing variadic-hook
// constructor signature (which bootstrap relies on).
func (s *GateService) WithAuditor(a GateAuditor) *GateService {
	if a != nil {
		s.auditor = a
	}
	return s
}

// WithAdvancer returns a new GateService with the given advancer attached.
func (s *GateService) WithAdvancer(a GatePipelineAdvancer) *GateService {
	if a != nil {
		s.advancer = a
	}
	return s
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
	return s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version)
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

// ApproveAndAdvance runs the standard Approve flow, then re-computes the
// sha256 of the provided contentBytes and compares it against the
// ArtifactHash recorded on the most recent gate event for the pipeline that
// carries an artifact hash (typically the submission event). If the hashes
// disagree, the downstream stage is NOT advanced and the block is recorded
// in the audit log. This guards against TOCTOU tampering of an artifact
// between the time the gate event was originally recorded and the time
// downstream stages pick it up.
//
// When the artifact hash matches (or no prior hash exists), and an advancer
// has been wired in, GatePipelineAdvancer.AdvanceAfterGate is invoked to
// trigger downstream-stage progression. If no advancer has been wired in
// (e.g. via the bootstrap default path), the method returns nil after a
// successful verification — the caller's own Approve path is responsible
// for downstream progression in that case.
func (s *GateService) ApproveAndAdvance(ctx context.Context, pipelineID, stage, approver, comment string, contentBytes []byte) error {
	if err := s.Approve(ctx, pipelineID, stage, approver, domain.GateChecklist{}, comment); err != nil {
		return err
	}

	events, err := s.gateRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("ApproveAndAdvance: list gate events for %s: %w", pipelineID, err)
	}
	if len(events) == 0 {
		return fmt.Errorf("ApproveAndAdvance: no gate_event found after Approve for %s/%s", pipelineID, stage)
	}

	// Walk back to the most recent event that actually carries an artifact
	// hash. The newly recorded Approve event won't have one (Approve creates
	// a fresh event with only ContentHash); the hash is recorded upstream
	// at submission time.
	var artifactEvent *domain.GateEvent
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].ArtifactHash != "" {
			artifactEvent = events[i]
			break
		}
	}
	if artifactEvent == nil {
		// No prior artifact hash to verify against — legacy path, allow.
		if s.advancer != nil {
			return s.advancer.AdvanceAfterGate(ctx, pipelineID, stage)
		}
		return nil
	}

	if err := artifactEvent.VerifyArtifactHash(contentBytes); err != nil {
		if s.auditor != nil {
			s.auditor.Log(ctx, GateAuditEvent{
				EventID: fmt.Sprintf("toctou-%s-%s-%d", pipelineID, stage, time.Now().UnixNano()),
				Actor:   approver,
				Action:  "gate_toctou_blocked",
				Details: err.Error(),
			})
		}
		return err
	}

	if s.advancer != nil {
		return s.advancer.AdvanceAfterGate(ctx, pipelineID, stage)
	}
	return nil
}
