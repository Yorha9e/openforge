package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"openforge/internal/agent/domain"
	agentport "openforge/internal/agent/port"
	"openforge/internal/auth/service"
	"openforge/internal/pipeline/port"
	"openforge/internal/shared/profile"
)

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
		engines:          make(map[string]*domain.QueryEngine),
		streamCancels:    make(map[string]context.CancelFunc),
		dispatchServices: &dispatchServices{},
	}
	c.writer = w
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
func newEngineWith() *domain.QueryEngine {
	qe := domain.NewQueryEngine(nil, agentport.LLMConfig{}, nil, domain.PipelineContext{PipelineID: "p-1"})
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
