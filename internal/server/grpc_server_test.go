package server

import (
	"context"
	"net"
	"net/http"
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"
	"openforge/internal/shared/profile"

	"connectrpc.com/connect"
)

// TestGRPCServer_CoordinatorCreateAgent verifies that the gRPC mux wires the
// CoordinatorServiceHandler such that a CreateAgent RPC returns a non-empty
// agentId. Path C: the server side is a stub — the test asserts the wire is
// end-to-end reachable.
func TestGRPCServer_CoordinatorCreateAgent(t *testing.T) {
	of := &profile.OpenForge{Config: &profile.Config{}}
	ts := newGRPCTestServer(t, of)
	defer ts.Close()

	client := agentv1connect.NewCoordinatorServiceClient(ts.Client(), ts.URL)
	req := connect.NewRequest(&agentv1.CreateAgentRequest{
		AgentId:    "agent-test-1",
		PipelineId: "pipe-1",
		ProjectId:  "proj-1",
	})
	resp, err := client.CreateAgent(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateAgent: unexpected error: %v", err)
	}
	if resp.Msg.AgentId == "" {
		t.Errorf("CreateAgent: expected non-empty agentId, got %q", resp.Msg.AgentId)
	}
}

// TestGRPCServer_GateApprove verifies the GateServiceHandler is wired and
// returns success=true for a basic Approve call.
func TestGRPCServer_GateApprove(t *testing.T) {
	of := &profile.OpenForge{Config: &profile.Config{}}
	ts := newGRPCTestServer(t, of)
	defer ts.Close()

	client := agentv1connect.NewGateServiceClient(ts.Client(), ts.URL)
	req := connect.NewRequest(&agentv1.GateApproveRequest{
		PipelineId: "pipe-1",
		Stage:      agentv1.StageType_STAGE_TYPE_IMPL,
		Actor:      "tester@example.com",
		Checklist: &agentv1.GateChecklist{
			CodeReviewed:      true,
			SecurityChecked:   true,
			LicenseCleared:    true,
			CodingStandardMet: true,
		},
	})
	resp, err := client.Approve(context.Background(), req)
	if err != nil {
		t.Fatalf("Approve: unexpected error: %v", err)
	}
	if !resp.Msg.Success {
		t.Errorf("Approve: expected success=true, got %+v", resp.Msg)
	}
}

// grpcTestServer is a minimal in-process gRPC test server. It runs
// StartGRPCServer on a random local port and exposes the URL to clients.
type grpcTestServer struct {
	server *http.Server
	URL    string
	hc     *http.Client
	ln     net.Listener
}

func newGRPCTestServer(t *testing.T, of *profile.OpenForge) *grpcTestServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	handler := NewGRPCMux(of)
	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(ln)
	}()
	ts := &grpcTestServer{
		server: srv,
		URL:    "http://" + ln.Addr().String(),
		hc:     &http.Client{},
		ln:     ln,
	}
	return ts
}

func (g *grpcTestServer) Client() *http.Client { return g.hc }

func (g *grpcTestServer) Close() {
	_ = g.server.Close()
	_ = g.ln.Close()
}
