package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"openforge/internal/auth/service"
	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
	pipeservice "openforge/internal/pipeline/service"
	"openforge/internal/shared/profile"
)

// --- mock GateRepository for T1: gate.approve dispatches to GateSvc.Approve ---

type mockGateRepo struct {
	mu             sync.Mutex
	events         []*domain.GateEvent
	latestHash     string
	createEventErr error
}

func (m *mockGateRepo) GetLatestHash(_ context.Context, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latestHash, nil
}

func (m *mockGateRepo) CreateEvent(_ context.Context, ev *domain.GateEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createEventErr != nil {
		return m.createEventErr
	}
	m.events = append(m.events, ev)
	m.latestHash = ev.ContentHash
	return nil
}

func (m *mockGateRepo) ListByPipeline(_ context.Context, pipelineID string) ([]*domain.GateEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.GateEvent
	for _, ev := range m.events {
		if ev.PipelineID == pipelineID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *mockGateRepo) ListPending(_ context.Context, _ string) ([]*domain.GateEvent, error) {
	return nil, nil
}

func (m *mockGateRepo) Claim(_ context.Context, _, _, _ string, _ time.Duration) error {
	return nil
}

func (m *mockGateRepo) ReleaseClaim(_ context.Context, _, _, _ string) error {
	return nil
}

// --- mock PipelineRepository for T1 ---

type mockPipelineRepoForGate struct {
	mu        sync.Mutex
	pipelines map[string]*domain.Pipeline
}

func (m *mockPipelineRepoForGate) Create(_ context.Context, p *domain.Pipeline) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pipelines == nil {
		m.pipelines = map[string]*domain.Pipeline{}
	}
	m.pipelines[p.ID] = p
	return nil
}

func (m *mockPipelineRepoForGate) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pipelines[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockPipelineRepoForGate) ListByProject(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}

func (m *mockPipelineRepoForGate) UpdateStatus(_ context.Context, _, _ string, _ int) error {
	return nil
}

func (m *mockPipelineRepoForGate) IncrementBacktrack(_ context.Context, _ string) error {
	return nil
}

func (m *mockPipelineRepoForGate) Delete(_ context.Context, _ string) error {
	return nil
}

func newL3PipelineAwaitingReview(id string) *domain.Pipeline {
	return &domain.Pipeline{
		ID:           id,
		ProjectID:    "proj1",
		Title:        "Test",
		Level:        "L3",
		Status:       "awaiting_review",
		CurrentStage: "impl",
		Version:      1,
		Stages: []domain.Stage{
			{Type: "clarify", Status: "passed"},
			{Type: "decompose", Status: "passed"},
			{Type: "impl", Status: "running"},
			{Type: "test", Status: "pending"},
			{Type: "deploy", Status: "pending"},
			{Type: "verify", Status: "pending"},
		},
	}
}

func TestChatWSRejectsMissingToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	if rec.Code == 101 {
		t.Fatalf("missing token must not upgrade")
	}
	if rec.Code != 401 {
		t.Fatalf("expected 401 for missing token, got %d", rec.Code)
	}
}

func TestChatWSRejectsInvalidToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	if rec.Code == 101 {
		t.Fatalf("invalid token must not upgrade")
	}
	if rec.Code != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestBearerTokenFromSubprotocols(t *testing.T) {
	got := bearerTokenFromSubprotocols([]string{"openforge.auth, bearer.abc.def.ghi"})
	if got != "abc.def.ghi" {
		t.Fatalf("token = %q, want abc.def.ghi", got)
	}
}

// TestWSGateApprove_CallsGateSvcApprove verifies that the gate.approve
// wsMessage dispatches to c.of.GateSvc.Approve (rather than the legacy
// stub that just wrote a notification). After the dispatch, the in-memory
// gate repo must contain a domain.GateEvent with Decision="approve" and
// the supplied Actor.
func TestWSGateApprove_CallsGateSvcApprove(t *testing.T) {
	gateRepo := &mockGateRepo{}
	pipeRepo := &mockPipelineRepoForGate{
		pipelines: map[string]*domain.Pipeline{
			"p-1": newL3PipelineAwaitingReview("p-1"),
		},
	}
	// Compile-time interface assertions
	var _ port.GateRepository = gateRepo
	var _ port.PipelineRepository = pipeRepo

	of := &profile.OpenForge{
		GateSvc: pipeservice.NewGateService(gateRepo, pipeRepo),
	}

	conn := &wsConn{
		of:    of,
		userID: "u-1",
	}

	// Captured writer
	var captured []map[string]any
	var mu sync.Mutex
	write := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		// JSON round-trip so we can assert on structure regardless of input
		b, _ := json.Marshal(v)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		captured = append(captured, m)
	}

	payload, err := json.Marshal(map[string]string{
		"pipeline_id": "p-1",
		"stage":       "impl",
		"approver":    "u-2",
		"comment":     "LGTM",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	conn.dispatch(wsMessage{Type: "gate.approve", Payload: payload}, write)

	// Assert: a gate.notify message was sent to the writer
	mu.Lock()
	defer mu.Unlock()
	if len(captured) == 0 {
		t.Fatalf("expected at least one message written, got 0")
	}
	foundNotify := false
	for _, m := range captured {
		if t2, _ := m["type"].(string); t2 == "gate.notify" {
			foundNotify = true
			break
		}
	}
	if !foundNotify {
		t.Fatalf("expected gate.notify message, got %+v", captured)
	}

	// Assert: the in-memory gate repo received a CreateEvent call with
	// Decision="approve" and Actor="u-2"
	gateRepo.mu.Lock()
	defer gateRepo.mu.Unlock()
	if len(gateRepo.events) == 0 {
		t.Fatalf("expected at least one gate event to be created, got 0")
	}
	ev := gateRepo.events[0]
	if ev.Decision != "approve" {
		t.Errorf("decision = %q, want %q", ev.Decision, "approve")
	}
	if ev.Actor != "u-2" {
		t.Errorf("actor = %q, want %q", ev.Actor, "u-2")
	}
	if ev.Event != "approved" {
		t.Errorf("event = %q, want %q", ev.Event, "approved")
	}
	if ev.PipelineID != "p-1" {
		t.Errorf("pipelineID = %q, want %q", ev.PipelineID, "p-1")
	}
	if ev.Stage != "impl" {
		t.Errorf("stage = %q, want %q", ev.Stage, "impl")
	}
	if ev.SummaryFeedback != "LGTM" {
		t.Errorf("summary = %q, want %q", ev.SummaryFeedback, "LGTM")
	}
}

// TestWSGateReject_CallsGateSvcReject verifies that the gate.reject
// wsMessage dispatches to c.of.GateSvc.Reject (rather than the legacy stub
// that just wrote a notification). After dispatch, the in-memory gate repo
// must contain a domain.GateEvent with Decision="reject" and the supplied
// actor/reason captured as the summary feedback.
func TestWSGateReject_CallsGateSvcReject(t *testing.T) {
	gateRepo := &mockGateRepo{}
	pipeRepo := &mockPipelineRepoForGate{
		pipelines: map[string]*domain.Pipeline{
			"p-2": newL3PipelineAwaitingReview("p-2"),
		},
	}

	of := &profile.OpenForge{
		GateSvc: pipeservice.NewGateService(gateRepo, pipeRepo),
	}

	conn := &wsConn{
		of:     of,
		userID: "u-1",
	}

	// Captured writer
	var captured []map[string]any
	var mu sync.Mutex
	write := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := json.Marshal(v)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		captured = append(captured, m)
	}

	payload, err := json.Marshal(map[string]string{
		"pipeline_id": "p-2",
		"stage":       "impl",
		"approver":    "u-2",
		"reason":      "tests failing",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	conn.dispatch(wsMessage{Type: "gate.reject", Payload: payload}, write)

	// Assert: a gate.notify message was sent to the writer
	mu.Lock()
	defer mu.Unlock()
	if len(captured) == 0 {
		t.Fatalf("expected at least one message written, got 0")
	}
	foundNotify := false
	for _, m := range captured {
		if t2, _ := m["type"].(string); t2 == "gate.notify" {
			foundNotify = true
			payload, _ := m["payload"].(map[string]any)
			if pl, _ := payload["pipeline_id"].(string); pl != "p-2" {
				t.Errorf("notify pipeline_id = %q, want %q", pl, "p-2")
			}
			if ev, _ := payload["event"].(string); ev != "rejected" {
				t.Errorf("notify event = %q, want %q", ev, "rejected")
			}
			break
		}
	}
	if !foundNotify {
		t.Fatalf("expected gate.notify message, got %+v", captured)
	}

	// Assert: the in-memory gate repo received a CreateEvent call with
	// Decision="reject" and Actor="u-2"
	gateRepo.mu.Lock()
	defer gateRepo.mu.Unlock()
	if len(gateRepo.events) == 0 {
		t.Fatalf("expected at least one gate event to be created, got 0")
	}
	ev := gateRepo.events[0]
	if ev.Decision != "reject" {
		t.Errorf("decision = %q, want %q", ev.Decision, "reject")
	}
	if ev.Actor != "u-2" {
		t.Errorf("actor = %q, want %q", ev.Actor, "u-2")
	}
	if ev.Event != "rejected" {
		t.Errorf("event = %q, want %q", ev.Event, "rejected")
	}
	if ev.PipelineID != "p-2" {
		t.Errorf("pipelineID = %q, want %q", ev.PipelineID, "p-2")
	}
	if ev.Stage != "impl" {
		t.Errorf("stage = %q, want %q", ev.Stage, "impl")
	}
	if ev.SummaryFeedback != "tests failing" {
		t.Errorf("summary = %q, want %q (reason should be stored as feedback)", ev.SummaryFeedback, "tests failing")
	}
}
