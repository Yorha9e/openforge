package adapter

import (
	"context"
	"fmt"
	"net/http"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"

	"connectrpc.com/connect"
)

// GateClient is a thin ConnectRPC client wrapper around the auto-generated
// agentv1connect.GateServiceClient. Path C wires the gRPC GateService
// handler in internal/server/grpc_server.go; this client allows BFF code
// (and internal pipeline service code, see T1 plan step "internal/pipeline/
// service/gate_service.go" to call gRPC gate client) to invoke gate
// operations over the wire instead of going through the in-process
// pipeline.Service.GateService directly.
//
// Future callers: BFF HTTP handlers and the pipeline service that
// currently drives the gate via in-process function calls.
type GateClient struct {
	httpClient *http.Client
	client     agentv1connect.GateServiceClient
}

// NewGateClient creates a ConnectRPC client targeting addr (e.g.
// "http://127.0.0.1:50051"). Pass the same addr that the gRPC server is
// bound to in StartGRPCServer.
func NewGateClient(addr string) (*GateClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("gate addr is empty")
	}
	baseURL := fmt.Sprintf("http://%s", addr)
	httpClient := &http.Client{}
	return &GateClient{
		httpClient: httpClient,
		client:     agentv1connect.NewGateServiceClient(httpClient, baseURL),
	}, nil
}

// Approve calls GateService.Approve.
func (c *GateClient) Approve(ctx context.Context, req *agentv1.GateApproveRequest) (*agentv1.GateApproveResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.Approve(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect gate approve: %w", err)
	}
	return resp.Msg, nil
}

// Reject calls GateService.Reject.
func (c *GateClient) Reject(ctx context.Context, req *agentv1.GateRejectRequest) (*agentv1.GateRejectResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.Reject(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect gate reject: %w", err)
	}
	return resp.Msg, nil
}

// Claim calls GateService.Claim.
func (c *GateClient) Claim(ctx context.Context, req *agentv1.GateClaimRequest) (*agentv1.GateClaimResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.Claim(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect gate claim: %w", err)
	}
	return resp.Msg, nil
}

// GetInbox calls GateService.GetInbox.
func (c *GateClient) GetInbox(ctx context.Context, req *agentv1.GateGetInboxRequest) (*agentv1.GateGetInboxResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.GetInbox(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect gate inbox: %w", err)
	}
	return resp.Msg, nil
}

// Close releases idle HTTP connections.
func (c *GateClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
