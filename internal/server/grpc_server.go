package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"
	"openforge/internal/shared/profile"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewGRPCMux builds an http.Handler that wires all 4 Agent v1 Connect-RPC
// services that live on the Go side (Coordinator, Gate) plus the 2 services
// that the Go side *consumes* (ToolRegistry, LLMRouter — those run on the
// Node.js IO process and are registered on the Node server in T1).
//
// Path C: the Go side serves CoordinatorService and GateService as stubs that
// answer shape-correct responses so the wire is verified end-to-end; full
// business logic for these lands in T2-T5. ToolRegistryService and
// LLMRouterService are exposed as noop stubs on the Go side so that Go-side
// service discovery / health probes do not 404 — the real client wiring
// uses the grpc_*_client.go wrappers in internal/agent/adapter/ to call
// cfg.GRPC.NodejsIOAddr.
func NewGRPCMux(of *profile.OpenForge) http.Handler {
	mux := http.NewServeMux()

	// CoordinatorService — Go 协调层 owns the handler.
	coordPath, coordHandler := agentv1connect.NewCoordinatorServiceHandler(
		newCoordinatorHandler(of),
	)
	mux.Handle(coordPath, coordHandler)

	// GateService — Go 协调层 owns the handler.
	gatePath, gateHandler := agentv1connect.NewGateServiceHandler(
		newGateHandler(of),
	)
	mux.Handle(gatePath, gateHandler)

	// ToolRegistryService and LLMRouterService run on Node.js IO; the Go
	// side exposes noop stubs for service-discovery / health probes.
	toolsPath, toolsHandler := agentv1connect.NewToolRegistryServiceHandler(
		newToolRegistryStub(),
	)
	mux.Handle(toolsPath, toolsHandler)

	llmPath, llmHandler := agentv1connect.NewLLMRouterServiceHandler(
		newLLMRouterStub(),
	)
	mux.Handle(llmPath, llmHandler)

	// h2c.NewHandler wraps the mux so gRPC clients (which require HTTP/2 with
	// prior knowledge) can connect over plaintext TCP. Connect protocol works
	// over both HTTP/1.1 and HTTP/2 — h2c ensures we serve HTTP/2 for gRPC.
	return h2c.NewHandler(mux, &http2.Server{})
}

// StartGRPCServer binds on addr (e.g. ":50051") and serves the gRPC mux
// until the process exits. Intended to be launched from a goroutine in
// Bootstrap so the gRPC endpoints are co-located with the HTTP server.
func StartGRPCServer(of *profile.OpenForge, addr string) error {
	handler := NewGRPCMux(of)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	slog.Info("OpenForge gRPC server starting", "addr", addr)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Coordinator service handler (stub — full logic lands in T2)
// ---------------------------------------------------------------------------

type coordinatorHandler struct {
	of *profile.OpenForge
}

func newCoordinatorHandler(of *profile.OpenForge) *coordinatorHandler {
	return &coordinatorHandler{of: of}
}

func (h *coordinatorHandler) CreateAgent(ctx context.Context, req *connect.Request[agentv1.CreateAgentRequest]) (*connect.Response[agentv1.CreateAgentResponse], error) {
	return connect.NewResponse(&agentv1.CreateAgentResponse{
		AgentId: req.Msg.AgentId,
		Status:  "created",
	}), nil
}

func (h *coordinatorHandler) DestroyAgent(ctx context.Context, req *connect.Request[agentv1.DestroyAgentRequest]) (*connect.Response[agentv1.DestroyAgentResponse], error) {
	return connect.NewResponse(&agentv1.DestroyAgentResponse{Success: true}), nil
}

func (h *coordinatorHandler) ExecuteStage(ctx context.Context, req *connect.Request[agentv1.ExecuteStageRequest], stream *connect.ServerStream[agentv1.ExecuteStageEvent]) error {
	return stream.Send(&agentv1.ExecuteStageEvent{EventType: "done"})
}

func (h *coordinatorHandler) Chat(ctx context.Context, req *connect.Request[agentv1.ChatRequest], stream *connect.ServerStream[agentv1.ChatEvent]) error {
	return stream.Send(&agentv1.ChatEvent{EventType: agentv1.ChatEventType_CHAT_EVENT_TYPE_DONE, IsDone: true})
}

func (h *coordinatorHandler) EditMessage(ctx context.Context, req *connect.Request[agentv1.EditMessageRequest]) (*connect.Response[agentv1.EditMessageResponse], error) {
	return connect.NewResponse(&agentv1.EditMessageResponse{Success: true, NewBranchId: req.Msg.BranchId + "-edit"}), nil
}

func (h *coordinatorHandler) StopGeneration(ctx context.Context, req *connect.Request[agentv1.StopGenerationRequest]) (*connect.Response[agentv1.StopGenerationResponse], error) {
	return connect.NewResponse(&agentv1.StopGenerationResponse{Success: true}), nil
}

func (h *coordinatorHandler) PauseGeneration(ctx context.Context, req *connect.Request[agentv1.PauseGenerationRequest]) (*connect.Response[agentv1.PauseGenerationResponse], error) {
	return connect.NewResponse(&agentv1.PauseGenerationResponse{Success: true}), nil
}

func (h *coordinatorHandler) ResumeGeneration(ctx context.Context, req *connect.Request[agentv1.ResumeGenerationRequest], stream *connect.ServerStream[agentv1.ChatEvent]) error {
	return stream.Send(&agentv1.ChatEvent{EventType: agentv1.ChatEventType_CHAT_EVENT_TYPE_DONE, IsDone: true})
}

func (h *coordinatorHandler) RegenerateFrom(ctx context.Context, req *connect.Request[agentv1.RegenerateFromRequest], stream *connect.ServerStream[agentv1.ChatEvent]) error {
	return stream.Send(&agentv1.ChatEvent{EventType: agentv1.ChatEventType_CHAT_EVENT_TYPE_DONE, IsDone: true})
}

func (h *coordinatorHandler) GetPipeline(ctx context.Context, req *connect.Request[agentv1.GetPipelineRequest]) (*connect.Response[agentv1.GetPipelineResponse], error) {
	return connect.NewResponse(&agentv1.GetPipelineResponse{PipelineId: req.Msg.PipelineId}), nil
}

func (h *coordinatorHandler) CancelPipeline(ctx context.Context, req *connect.Request[agentv1.CancelPipelineRequest]) (*connect.Response[agentv1.CancelPipelineResponse], error) {
	return connect.NewResponse(&agentv1.CancelPipelineResponse{
		Success:     true,
		FinalStatus: agentv1.PipelineStatus_PIPELINE_STATUS_CANCELLED,
	}), nil
}

func (h *coordinatorHandler) ModifyPipelineScope(ctx context.Context, req *connect.Request[agentv1.ModifyPipelineScopeRequest]) (*connect.Response[agentv1.ModifyPipelineScopeResponse], error) {
	return connect.NewResponse(&agentv1.ModifyPipelineScopeResponse{Success: true}), nil
}

func (h *coordinatorHandler) PushTokenUsage(ctx context.Context, stream *connect.ClientStream[agentv1.TokenUsageEvent]) (*connect.Response[agentv1.TokenUsageAck], error) {
	return connect.NewResponse(&agentv1.TokenUsageAck{Success: true}), nil
}

func (h *coordinatorHandler) Health(ctx context.Context, req *connect.Request[agentv1.HealthRequest]) (*connect.Response[agentv1.HealthResponse], error) {
	return connect.NewResponse(&agentv1.HealthResponse{Serving: true}), nil
}

// ---------------------------------------------------------------------------
// Gate service handler (stub — full logic lands in T5)
// ---------------------------------------------------------------------------

type gateHandler struct {
	of *profile.OpenForge
}

func newGateHandler(of *profile.OpenForge) *gateHandler {
	return &gateHandler{of: of}
}

func (h *gateHandler) Approve(ctx context.Context, req *connect.Request[agentv1.GateApproveRequest]) (*connect.Response[agentv1.GateApproveResponse], error) {
	return connect.NewResponse(&agentv1.GateApproveResponse{
		Success:    true,
		NextStatus: agentv1.PipelineStatus_PIPELINE_STATUS_RUNNING,
	}), nil
}

func (h *gateHandler) Reject(ctx context.Context, req *connect.Request[agentv1.GateRejectRequest]) (*connect.Response[agentv1.GateRejectResponse], error) {
	return connect.NewResponse(&agentv1.GateRejectResponse{Success: true}), nil
}

func (h *gateHandler) Claim(ctx context.Context, req *connect.Request[agentv1.GateClaimRequest]) (*connect.Response[agentv1.GateClaimResponse], error) {
	return connect.NewResponse(&agentv1.GateClaimResponse{Success: true}), nil
}

func (h *gateHandler) GetInbox(ctx context.Context, req *connect.Request[agentv1.GateGetInboxRequest]) (*connect.Response[agentv1.GateGetInboxResponse], error) {
	return connect.NewResponse(&agentv1.GateGetInboxResponse{}), nil
}

// ---------------------------------------------------------------------------
// ToolRegistry + LLMRouter noop stubs (Node.js IO owns the real impls; the
// Go side exposes these so probes and discovery do not 404.)
// ---------------------------------------------------------------------------

type toolRegistryStub struct{}

func newToolRegistryStub() *toolRegistryStub { return &toolRegistryStub{} }

func (s *toolRegistryStub) SearchTools(ctx context.Context, req *connect.Request[agentv1.SearchToolsRequest]) (*connect.Response[agentv1.SearchToolsResponse], error) {
	return connect.NewResponse(&agentv1.SearchToolsResponse{}), nil
}

func (s *toolRegistryStub) CallTool(ctx context.Context, req *connect.Request[agentv1.CallToolRequest]) (*connect.Response[agentv1.CallToolResponse], error) {
	return connect.NewResponse(&agentv1.CallToolResponse{ToolName: req.Msg.ToolName}), nil
}

func (s *toolRegistryStub) CallToolStream(ctx context.Context, req *connect.Request[agentv1.CallToolRequest], stream *connect.ServerStream[agentv1.CallToolStreamChunk]) error {
	return stream.Send(&agentv1.CallToolStreamChunk{EventType: "done", IsDone: true})
}

func (s *toolRegistryStub) RegisterTool(ctx context.Context, req *connect.Request[agentv1.RegisterToolRequest]) (*connect.Response[agentv1.RegisterToolResponse], error) {
	return connect.NewResponse(&agentv1.RegisterToolResponse{Success: true, ToolName: req.Msg.Tool.Name}), nil
}

func (s *toolRegistryStub) UnregisterTool(ctx context.Context, req *connect.Request[agentv1.UnregisterToolRequest]) (*connect.Response[agentv1.UnregisterToolResponse], error) {
	return connect.NewResponse(&agentv1.UnregisterToolResponse{Success: true}), nil
}

func (s *toolRegistryStub) ListAllTools(ctx context.Context, req *connect.Request[agentv1.ListAllToolsRequest]) (*connect.Response[agentv1.ListAllToolsResponse], error) {
	return connect.NewResponse(&agentv1.ListAllToolsResponse{TotalCount: 0}), nil
}

func (s *toolRegistryStub) RebuildIndex(ctx context.Context, req *connect.Request[agentv1.RebuildIndexRequest]) (*connect.Response[agentv1.RebuildIndexResponse], error) {
	return connect.NewResponse(&agentv1.RebuildIndexResponse{Success: true, Status: "queued"}), nil
}

func (s *toolRegistryStub) GetIndexStatus(ctx context.Context, req *connect.Request[agentv1.GetIndexStatusRequest]) (*connect.Response[agentv1.GetIndexStatusResponse], error) {
	return connect.NewResponse(&agentv1.GetIndexStatusResponse{Status: "ready"}), nil
}

type llmRouterStub struct{}

func newLLMRouterStub() *llmRouterStub { return &llmRouterStub{} }

func (s *llmRouterStub) Chat(ctx context.Context, req *connect.Request[agentv1.LLMChatRequest]) (*connect.Response[agentv1.LLMChatResponse], error) {
	return connect.NewResponse(&agentv1.LLMChatResponse{Id: "stub", StopReason: "end_turn"}), nil
}

func (s *llmRouterStub) ChatStream(ctx context.Context, req *connect.Request[agentv1.LLMChatRequest], stream *connect.ServerStream[agentv1.LLMChatStreamChunk]) error {
	return stream.Send(&agentv1.LLMChatStreamChunk{EventType: "done"})
}

func (s *llmRouterStub) ListModels(ctx context.Context, req *connect.Request[agentv1.ListModelsRequest]) (*connect.Response[agentv1.ListModelsResponse], error) {
	return connect.NewResponse(&agentv1.ListModelsResponse{}), nil
}

func (s *llmRouterStub) SwitchModel(ctx context.Context, req *connect.Request[agentv1.SwitchModelRequest]) (*connect.Response[agentv1.SwitchModelResponse], error) {
	return connect.NewResponse(&agentv1.SwitchModelResponse{Success: true}), nil
}

func (s *llmRouterStub) GetTokenUsage(ctx context.Context, req *connect.Request[agentv1.GetTokenUsageRequest]) (*connect.Response[agentv1.GetTokenUsageResponse], error) {
	return connect.NewResponse(&agentv1.GetTokenUsageResponse{}), nil
}

func (s *llmRouterStub) RecordTokenUsage(ctx context.Context, req *connect.Request[agentv1.RecordTokenUsageRequest]) (*connect.Response[agentv1.RecordTokenUsageResponse], error) {
	// Go-side stub: in production this method is owned by Node.js IO;
	// the Go side only needs to satisfy the interface for
	// service-discovery probes, so we return zeros.
	_ = req
	return connect.NewResponse(&agentv1.RecordTokenUsageResponse{Inserted: 0, Skipped: 0}), nil
}
