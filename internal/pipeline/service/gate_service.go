package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	agentv1 "openforge/gen/go/agent/v1"
	agentadapter "openforge/internal/agent/adapter"
	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type GateService struct {
	gateRepo port.GateRepository
	pipeRepo port.PipelineRepository
	hooks    domain.HookChain
	// grpcClient, when non-nil, makes Approve/Reject round-trip through the
	// gRPC GateService handler (Path C wire). When nil, the in-process
	// path runs (legacy behavior preserved for tests that do not start
	// the gRPC server).
	grpcClient *agentadapter.GateClient
}

func NewGateService(gateRepo port.GateRepository, pipeRepo port.PipelineRepository, hooks ...domain.GateHook) *GateService {
	return &GateService{
		gateRepo: gateRepo,
		pipeRepo: pipeRepo,
		hooks:    hooks,
	}
}

// WithGRPCClient attaches a gRPC GateService client. Path C: when set,
// Approve/Reject go through the wire instead of the in-process path. The
// caller (Bootstrap) is expected to construct the gRPC client and pass it
// after the gRPC server has been started.
func (s *GateService) WithGRPCClient(c *agentadapter.GateClient) *GateService {
	s.grpcClient = c
	return s
}

func (s *GateService) Approve(ctx context.Context, pipelineID, stage, actor string, checklist domain.GateChecklist, summary string) error {
	if s.grpcClient != nil {
		_, err := s.grpcClient.Approve(ctx, &agentv1.GateApproveRequest{
			PipelineId: pipelineID,
			Stage:      domainStageToProtoStage(stage),
			Actor:      actor,
			Checklist: &agentv1.GateChecklist{
				CodeReviewed:      checklist.CodeReviewed,
				SecurityChecked:   checklist.SecurityChecked,
				LicenseCleared:    checklist.LicenseCleared,
				CodingStandardMet: checklist.CodingStandardMet,
			},
		})
		if err != nil {
			return fmt.Errorf("gRPC gate approve: %w", err)
		}
	}
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
	if s.grpcClient != nil {
		lc := make([]*agentv1.LineComment, len(comments))
		for i, c := range comments {
			lc[i] = &agentv1.LineComment{
				FilePath: c.FilePath,
				Line:     int32(c.Line),
				Comment:  c.Comment,
				Mark:     domainMarkToProto(c.Mark),
			}
		}
		_, err := s.grpcClient.Reject(ctx, &agentv1.GateRejectRequest{
			PipelineId:      pipelineID,
			Stage:           domainStageToProtoStage(stage),
			Actor:           actor,
			LineComments:    lc,
			SummaryFeedback: summary,
		})
		if err != nil {
			return fmt.Errorf("gRPC gate reject: %w", err)
		}
	}
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

func domainStageToProtoStage(stage string) agentv1.StageType {
	switch stage {
	case "clarify":
		return agentv1.StageType_STAGE_TYPE_CLARIFY
	case "decompose":
		return agentv1.StageType_STAGE_TYPE_DECOMPOSE
	case "impl":
		return agentv1.StageType_STAGE_TYPE_IMPL
	case "test":
		return agentv1.StageType_STAGE_TYPE_TEST
	case "deploy":
		return agentv1.StageType_STAGE_TYPE_DEPLOY
	case "verify":
		return agentv1.StageType_STAGE_TYPE_VERIFY
	default:
		return agentv1.StageType_STAGE_TYPE_UNSPECIFIED
	}
}

func domainMarkToProto(mark string) agentv1.FileMark {
	switch mark {
	case "accept":
		return agentv1.FileMark_FILE_MARK_ACCEPT
	case "needs_revision":
		return agentv1.FileMark_FILE_MARK_NEEDS_REVISION
	case "needs_discussion":
		return agentv1.FileMark_FILE_MARK_NEEDS_DISCUSSION
	default:
		return agentv1.FileMark_FILE_MARK_UNSPECIFIED
	}
}
