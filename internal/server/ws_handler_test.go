package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	agentdomain "openforge/internal/agent/domain"
	agentport "openforge/internal/agent/port"
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

// captureWriter is a thread-safe sink for ws writes during tests.
type captureWriter struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (w *captureWriter) write(v any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m, ok := v.(map[string]any); ok {
		w.msgs = append(w.msgs, m)
	}
}

func (w *captureWriter) last() map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.msgs) == 0 {
		return nil
	}
	return w.msgs[len(w.msgs)-1]
}

// newConnForTest builds a wsConn whose write() is replaced by a capture.
// dispatchServices is wired so the test can control the 5 service hooks.
func newConnForTest() (*wsConn, *captureWriter) {
	w := &captureWriter{}
	c := &wsConn{
		of:               &profile.OpenForge{},
		engines:          make(map[string]*agentdomain.QueryEngine),
		streamCancels:    make(map[string]context.CancelFunc),
		dispatchServices: &dispatchServices{},
	}
	c.writer = w.write
	return c, w
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// newEngineWith returns a QueryEngine whose in-memory history contains one
// message with ID "msg-1" so the happy-path EditMessage test can find it.
func newEngineWith() *agentdomain.QueryEngine {
	qe := agentdomain.NewQueryEngine(nil, agentport.LLMConfig{}, nil, agentdomain.PipelineContext{PipelineID: "p-1"})
	qe.SetConversationRepo(stubConvRepo{})
	qe.LoadMessages([]agentport.Message{{ID: "msg-1", Role: "user", Content: "hello"}})
	return qe
}

// stubConvRepo is a no-op ConversationRepository implementation suitable for
// constructing a QueryEngine in tests without a real DB.
type stubConvRepo struct{}

func (stubConvRepo) SaveMessage(ctx context.Context, m *port.DBMessage) error { return nil }
func (stubConvRepo) BatchSaveMessages(ctx context.Context, msgs []*port.DBMessage) error {
	return nil
}
func (stubConvRepo) GetMessages(ctx context.Context, pipelineID, branchID string) ([]*port.DBMessage, error) {
	return nil, nil
}
func (stubConvRepo) CreateBranch(ctx context.Context, b *port.DBBranch) error { return nil }
func (stubConvRepo) GetBranch(ctx context.Context, branchID string) (*port.DBBranch, error) {
	return nil, nil
}
func (stubConvRepo) GetActiveBranch(ctx context.Context, pipelineID string) (*port.DBBranch, error) {
	return nil, nil
}
func (stubConvRepo) ListBranches(ctx context.Context, pipelineID string) ([]*port.DBBranch, error) {
	return nil, nil
}

// --- 11 handler tests ---

func TestWSChatEdit_UpdatesAndTriggersResend(t *testing.T) {
	c, w := newConnForTest()
	qe := newEngineWith()
	c.engines["p-1"] = qe

	c.dispatch(wsMessage{Type: "chat.edit", Payload: mustMarshal(map[string]any{
		"message_id": "msg-1",
		"content":    "updated",
	})})

	ack := w.last()
	if ack == nil || ack["type"] != "chat.edited" {
		t.Fatalf("expected chat.edited ack, got %#v", ack)
	}
}

func TestWSChatPause_SendsPauseAck(t *testing.T) {
	c, w := newConnForTest()
	c.dispatch(wsMessage{Type: "chat.pause"})
	if ack := w.last(); ack == nil || ack["type"] != "chat.paused" {
		t.Fatalf("expected chat.paused, got %#v", ack)
	}
}

func TestWSChatResume_ResumesStream(t *testing.T) {
	c, w := newConnForTest()
	c.dispatch(wsMessage{Type: "chat.resume"})
	if ack := w.last(); ack == nil || ack["type"] != "chat.resumed" {
		t.Fatalf("expected chat.resumed, got %#v", ack)
	}
}

func TestWSChatRetry_ResendsFromMessageID(t *testing.T) {
	c, w := newConnForTest()
	qe := newEngineWith()
	c.engines["p-1"] = qe

	c.dispatch(wsMessage{Type: "chat.retry", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"message_id":  "msg-1",
	})})

	if ack := w.last(); ack == nil || ack["type"] != "chat.retry_started" {
		t.Fatalf("expected chat.retry_started, got %#v", ack)
	}
}

func TestWSChatCancelBranch_CancelsActiveBranch(t *testing.T) {
	c, w := newConnForTest()
	called := ""
	c.dispatchServices.DeactivateBranch = func(ctx context.Context, branchID string) error {
		called = branchID
		return nil
	}

	c.dispatch(wsMessage{Type: "chat.cancel_branch", Payload: mustMarshal(map[string]any{
		"branch_id": "br-1",
	})})

	if called != "br-1" {
		t.Fatalf("expected DeactivateBranch(br-1), got %q", called)
	}
	if ack := w.last(); ack == nil || ack["type"] != "chat.branch_cancelled" {
		t.Fatalf("expected chat.branch_cancelled, got %#v", ack)
	}
}

func TestWSPipelineModifyScope_TriggersBacktrack(t *testing.T) {
	c, w := newConnForTest()
	called := ""
	c.dispatchServices.ModifyScope = func(ctx context.Context, id, req string) error {
		called = id + "|" + req
		return nil
	}

	c.dispatch(wsMessage{Type: "pipeline.modify_scope", Payload: mustMarshal(map[string]any{
		"pipeline_id":     "p-1",
		"new_requirement": "add more tests",
	})})

	if called != "p-1|add more tests" {
		t.Fatalf("expected ModifyScope(p-1, add more tests), got %q", called)
	}
	if ack := w.last(); ack == nil || ack["type"] != "pipeline.scope_modified" {
		t.Fatalf("expected pipeline.scope_modified, got %#v", ack)
	}
}

func TestWSModelSwitch_UpdatesActiveModel(t *testing.T) {
	c, w := newConnForTest()
	called := ""
	c.dispatchServices.SwitchModel = func(ctx context.Context, model string) error {
		called = model
		return nil
	}

	c.dispatch(wsMessage{Type: "model.switch", Payload: mustMarshal(map[string]any{
		"model": "gpt-4",
	})})

	if called != "gpt-4" {
		t.Fatalf("expected SwitchModel(gpt-4), got %q", called)
	}
	if ack := w.last(); ack == nil || ack["type"] != "model.switched" {
		t.Fatalf("expected model.switched, got %#v", ack)
	}
}

func TestWSTerminalInput_StreamsToSandbox(t *testing.T) {
	c, w := newConnForTest()
	called := ""
	c.dispatchServices.TerminalInput = func(ctx context.Context, pipelineID, input string) error {
		called = pipelineID + "|" + input
		return nil
	}

	c.dispatch(wsMessage{Type: "terminal.input", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"input":       "ls -la",
	})})

	if called != "p-1|ls -la" {
		t.Fatalf("expected TerminalInput(p-1, ls -la), got %q", called)
	}
	if ack := w.last(); ack == nil || ack["type"] != "terminal.input_acked" {
		t.Fatalf("expected terminal.input_acked, got %#v", ack)
	}
}

func TestWSPanelLayoutSave_PersistsUserLayout(t *testing.T) {
	c, w := newConnForTest()
	called := ""
	c.dispatchServices.SaveLayout = func(ctx context.Context, userID string, layout map[string]any) error {
		called = userID + ":" + layout["panel"].(string)
		return nil
	}

	c.dispatch(wsMessage{Type: "panel.layout.save", Payload: mustMarshal(map[string]any{
		"user_id": "u-1",
		"layout":  map[string]any{"panel": "left"},
	})})

	if called != "u-1:left" {
		t.Fatalf("expected SaveLayout(u-1, left), got %q", called)
	}
	if ack := w.last(); ack == nil || ack["type"] != "panel.layout_saved" {
		t.Fatalf("expected panel.layout_saved, got %#v", ack)
	}
}

// --- T5 sync.request / sync.replay tests ---

// TestWSSyncRequest_ReplaysMissedEvents verifies that on reconnect the client
// sends a sync.request with the last sequence number it consumed, and the
// server responds with one sync.replay message per missed event in the
// TraceStore.
func TestWSSyncRequest_ReplaysMissedEvents(t *testing.T) {
	c, w := newConnForTest()

	now := time.Now()
	c.of.TraceStore = &agentdomain.MemTraceStore{
		Data: map[string][]agentdomain.TraceEvent{
			"p-1": {
				{Seq: 1, PipelineID: "p-1", Event: "chat.stream", Payload: []byte(`{"delta":"a"}`), Timestamp: now.Add(-3 * time.Second)},
				{Seq: 2, PipelineID: "p-1", Event: "chat.stream", Payload: []byte(`{"delta":"b"}`), Timestamp: now.Add(-2 * time.Second)},
				{Seq: 3, PipelineID: "p-1", Event: "chat.stream_done", Payload: []byte(`{"content":"ok"}`), Timestamp: now.Add(-1 * time.Second)},
			},
		},
	}

	c.dispatch(wsMessage{Type: "sync.request", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"last_seq":    0,
	})})

	// Expect 3 sync.replay messages (one per missed event).
	var replays []map[string]any
	for _, m := range w.msgs {
		if m["type"] == "sync.replay" {
			replays = append(replays, m)
		}
	}
	if len(replays) != 3 {
		t.Fatalf("expected 3 sync.replay messages, got %d (all: %#v)", len(replays), w.msgs)
	}
	if replays[0]["type"] != "sync.replay" {
		t.Fatalf("first replay type wrong: %#v", replays[0])
	}
	// Verify payload shape
	p0, _ := replays[0]["payload"].(map[string]any)
	if p0["event"] != "chat.stream" {
		t.Fatalf("first replay event wrong: %v", p0["event"])
	}
}

// TestWSSyncRequest_NoTraceStore_ReturnsError verifies that when no
// TraceStore is wired the server emits a sync_failed error instead of
// silently dropping the request.
func TestWSSyncRequest_NoTraceStore_ReturnsError(t *testing.T) {
	c, w := newConnForTest()
	// No TraceStore on of.
	c.dispatch(wsMessage{Type: "sync.request", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"last_seq":    0,
	})})

	ack := w.last()
	if ack == nil || ack["type"] != "error" {
		t.Fatalf("expected error response, got %#v", ack)
	}
	payload, _ := ack["payload"].(map[string]any)
	if payload["code"] != "sync_failed" {
		t.Fatalf("expected code sync_failed, got %v", payload["code"])
	}
}

// TestWSSyncRequest_LastSeqFiltersStaleEvents verifies that events with
// seq <= lastSeq are not replayed.
func TestWSSyncRequest_LastSeqFiltersStaleEvents(t *testing.T) {
	c, w := newConnForTest()

	now := time.Now()
	c.of.TraceStore = &agentdomain.MemTraceStore{
		Data: map[string][]agentdomain.TraceEvent{
			"p-1": {
				{Seq: 1, PipelineID: "p-1", Event: "chat.stream", Payload: []byte(`{"delta":"a"}`), Timestamp: now.Add(-3 * time.Second)},
				{Seq: 2, PipelineID: "p-1", Event: "chat.stream", Payload: []byte(`{"delta":"b"}`), Timestamp: now.Add(-2 * time.Second)},
				{Seq: 3, PipelineID: "p-1", Event: "chat.stream_done", Payload: []byte(`{"content":"ok"}`), Timestamp: now.Add(-1 * time.Second)},
			},
		},
	}

	c.dispatch(wsMessage{Type: "sync.request", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"last_seq":    1,
	})})

	var replays []map[string]any
	for _, m := range w.msgs {
		if m["type"] == "sync.replay" {
			replays = append(replays, m)
		}
	}
	if len(replays) != 2 {
		t.Fatalf("expected 2 sync.replay (seq>1), got %d (all: %#v)", len(replays), w.msgs)
	}
}

func TestWSChatRetry_UnknownMessageID_ReturnsError(t *testing.T) {
	c, w := newConnForTest()
	// No engine registered for p-1, so dispatchChatRetry returns errNoSuchMessage.
	c.dispatch(wsMessage{Type: "chat.retry", Payload: mustMarshal(map[string]any{
		"pipeline_id": "p-1",
		"message_id":  "unknown-msg",
	})})

	ack := w.last()
	if ack == nil || ack["type"] != "error" {
		t.Fatalf("expected error response, got %#v", ack)
	}
	payload, _ := ack["payload"].(map[string]any)
	if payload["code"] != "retry_failed" {
		t.Fatalf("expected retry_failed, got %v", payload["code"])
	}
}

func TestWSChatEdit_NoSuchMessage_ReturnsError(t *testing.T) {
	c, w := newConnForTest()
	qe := newEngineWith() // contains only msg-1
	c.engines["p-1"] = qe

	c.dispatch(wsMessage{Type: "chat.edit", Payload: mustMarshal(map[string]any{
		"message_id": "missing",
		"content":    "new",
	})})

	ack := w.last()
	if ack == nil || ack["type"] != "error" {
		t.Fatalf("expected error response, got %#v", ack)
	}
	payload, _ := ack["payload"].(map[string]any)
	if payload["code"] != "edit_failed" {
		t.Fatalf("expected edit_failed, got %v", payload["code"])
	}
}

// --- original auth tests (kept) ---

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
		of:     of,
		userID: "u-1",
	}

	// Captured writer — wired through the test hook on wsConn.
	var captured []map[string]any
	var mu sync.Mutex
	conn.writer = func(v any) {
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

	conn.dispatch(wsMessage{Type: "gate.approve", Payload: payload})

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

	// Captured writer — wired through the test hook on wsConn.
	var captured []map[string]any
	var mu sync.Mutex
	conn.writer = func(v any) {
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

	conn.dispatch(wsMessage{Type: "gate.reject", Payload: payload})

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
