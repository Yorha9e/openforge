package adapter

import (
	"context"
	"fmt"
	"net/http"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"

	"connectrpc.com/connect"
)

// CoordinatorClient is a thin ConnectRPC client wrapper around the
// auto-generated agentv1connect.CoordinatorServiceClient. The Go 协调层
// (this process) is itself the server; the client wrapper exists so that
// other Go-side callers (BFF, test harness, internal services) can invoke
// the coordinator using the same wire format that the Node.js IO layer uses.
//
// Path C: this client is the calling interface used by the internal
// coordinator stub handler in internal/server/grpc_server.go and by future
// internal pipeline service code that needs to round-trip to a remote
// coordinator. It is intentionally minimal — only the methods required by
// Path C's wire-validation tests are exposed here.
type CoordinatorClient struct {
	httpClient *http.Client
	client     agentv1connect.CoordinatorServiceClient
}

// NewCoordinatorClient creates a ConnectRPC client targeting addr (e.g.
// "http://127.0.0.1:50051"). Pass the same addr that the gRPC server is
// bound to in StartGRPCServer.
func NewCoordinatorClient(addr string) (*CoordinatorClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("coordinator addr is empty")
	}
	baseURL := fmt.Sprintf("http://%s", addr)
	httpClient := &http.Client{}
	return &CoordinatorClient{
		httpClient: httpClient,
		client:     agentv1connect.NewCoordinatorServiceClient(httpClient, baseURL),
	}, nil
}

// CreateAgent calls CoordinatorService.CreateAgent.
func (c *CoordinatorClient) CreateAgent(ctx context.Context, req *agentv1.CreateAgentRequest) (*agentv1.CreateAgentResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.CreateAgent(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect create agent: %w", err)
	}
	return resp.Msg, nil
}

// DestroyAgent calls CoordinatorService.DestroyAgent.
func (c *CoordinatorClient) DestroyAgent(ctx context.Context, req *agentv1.DestroyAgentRequest) (*agentv1.DestroyAgentResponse, error) {
	connectReq := connect.NewRequest(req)
	resp, err := c.client.DestroyAgent(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect destroy agent: %w", err)
	}
	return resp.Msg, nil
}

// Health calls CoordinatorService.Health.
func (c *CoordinatorClient) Health(ctx context.Context) (*agentv1.HealthResponse, error) {
	resp, err := c.client.Health(ctx, connect.NewRequest(&agentv1.HealthRequest{}))
	if err != nil {
		return nil, fmt.Errorf("connect health: %w", err)
	}
	return resp.Msg, nil
}

// Close releases idle HTTP connections.
func (c *CoordinatorClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
