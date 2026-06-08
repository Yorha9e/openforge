package adapter

import (
	"context"
	"fmt"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"
	pipelineport "openforge/internal/pipeline/port"

	"connectrpc.com/connect"
)

// TokenUsageSink is the minimal port the LLM gRPC server needs from the
// repository layer. *pipelineadapter.PGRepository satisfies it via
// BatchRecordTokenUsage (added by T1).
type TokenUsageSink interface {
	BatchRecordTokenUsage(ctx context.Context, recs []pipelineport.TokenUsageRecord) error
}

// LLMRouterGRPCServer implements agentv1connect.LLMRouterServiceHandler.
// Only RecordTokenUsage is fully implemented; the other 5 RPCs are
// Unimplemented because the production handlers live in Node.js (port 50051).
// This server exists so the Node.js IO layer can flush its TokenMeter buffer
// into the Go coordinate layer's token_usage table via gRPC.
type LLMRouterGRPCServer struct {
	agentv1connect.UnimplementedLLMRouterServiceHandler
	repo TokenUsageSink
}

// NewLLMRouterGRPCServer returns a ConnectRPC handler that implements only
// RecordTokenUsage; the remaining 5 RPCs return CodeUnimplemented.
func NewLLMRouterGRPCServer(repo TokenUsageSink) *LLMRouterGRPCServer {
	return &LLMRouterGRPCServer{repo: repo}
}

// RecordTokenUsage converts the proto batch into port records and persists
// them in a single transaction. The proto-level record.Id is intentionally
// discarded: the DB's BIGSERIAL sequence generates the real id, and the
// token_usage table has no UNIQUE(id) constraint (composite PK with
// created_at). Caller-supplied id is a gRPC-correlation id, not a DB id.
func (s *LLMRouterGRPCServer) RecordTokenUsage(
	ctx context.Context,
	req *connect.Request[agentv1.RecordTokenUsageRequest],
) (*connect.Response[agentv1.RecordTokenUsageResponse], error) {
	if s.repo == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("token usage sink not configured"))
	}
	in := req.Msg.GetRecords()
	if len(in) == 0 {
		return connect.NewResponse(&agentv1.RecordTokenUsageResponse{Inserted: 0, Skipped: 0}), nil
	}
	recs := make([]pipelineport.TokenUsageRecord, len(in))
	for i, r := range in {
		createdAt := r.GetCreatedAt().AsTime()
		recs[i] = pipelineport.TokenUsageRecord{
			// ID intentionally NOT set — T1 PG impl does not use it.
			PipelineID:       r.GetPipelineId(),
			ProjectID:        r.GetProjectId(),
			Provider:         r.GetProvider(),
			Model:            r.GetModel(),
			PromptTokens:     r.GetPromptTokens(),
			CompletionTokens: r.GetCompletionTokens(),
			EstimatedCost:    r.GetEstimatedCost(),
			CreatedAt:        createdAt,
		}
	}
	if err := s.repo.BatchRecordTokenUsage(ctx, recs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("batch record token usage: %w", err))
	}
	return connect.NewResponse(&agentv1.RecordTokenUsageResponse{
		Inserted: int32(len(recs)),
		Skipped:  0,
	}), nil
}
