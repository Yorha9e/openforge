package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"openforge/internal/agent/domain"
	agentport "openforge/internal/agent/port"
	authadapter "openforge/internal/auth/adapter"
	"openforge/internal/auth/service"
	pipelinedomain "openforge/internal/pipeline/domain"
	"openforge/internal/shared/profile"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"openforge.auth"},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Allow localhost dev servers (5173, 5174) and empty origin
		if origin == "" {
			return true
		}
		for _, prefix := range []string{"http://localhost:", "http://127.0.0.1:"} {
			if len(origin) > len(prefix) && origin[:len(prefix)] == prefix {
				return true
			}
		}
		return false
	},
}

const (
	wsPingInterval = 30 * time.Second
	wsPongTimeout  = 10 * time.Second
	wsMaxPongFail  = 3
	wsAuthTimeout  = 5 * time.Second
)

var errMissingWebSocketToken = errors.New("missing websocket token")

type wsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type chatSendPayload struct {
	PipelineID string `json:"pipeline_id"`
	Message    string `json:"message"`
	WorkDir    string `json:"work_dir,omitempty"`
}

type authPayload struct {
	Token string `json:"token"`
}

type wsConn struct {
	conn               *websocket.Conn
	jwtSvc             *service.JWTService
	authRepo           *authadapter.PGAuthRepository
	userID             string
	userRole           string
	mu                 sync.Mutex
	engines            map[string]*domain.QueryEngine
	streamCancels      map[string]context.CancelFunc // pipelineID → cancel
	of                 *profile.OpenForge
	pongFail           int
	lastPipelineStage  string
	lastPipelineStatus string
	wsRPC              *domain.WSRPC
	writer             wsWriter          // test hook; when non-nil, write() routes through it
	dispatchServices   *dispatchServices // test hook; when nil, production wiring is used
	// traceCtx is the OTel trace context extracted from the inbound
	// W3C traceparent header at WS upgrade time.  Subsequent handlers
	// use it as the parent context for downstream gRPC calls to the
	// Node.js IO layer, so the entire Go↔Node session stays in one trace.
	traceCtx context.Context
}

// wsWriter is the test-facing capture sink for wsConn.write. Production
// code uses the default method on *wsConn (which writes to the real
// websocket.Conn).
type wsWriter = func(v any)

// dispatchServices holds the function pointers used by the 11 T4 WS cases
// (chat.edit / pause / resume / retry / cancel_branch, pipeline.modify_scope,
// model.switch, terminal.input, panel.layout.save). In production, these
// are wired by handleChatWS to call into the OpenForge fields. In tests,
// they are replaced wholesale to inject stubs without touching the real
// production types.
type dispatchServices struct {
	DeactivateBranch func(ctx context.Context, branchID string) error
	ModifyScope      func(ctx context.Context, pipelineID, newRequirement string) error
	SwitchModel      func(ctx context.Context, model string) error
	TerminalInput    func(ctx context.Context, pipelineID, input string) error
	SaveLayout       func(ctx context.Context, userID string, layout map[string]any) error
}

func handleChatWS(of *profile.OpenForge, jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := verifyWebSocketRequest(r, jwtSvc)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Extract W3C traceparent from the upgrade request via the global
		// OTel propagator (set by InitOTelTracer to W3C TraceContext+Baggage).
		// If no upstream trace context was sent (e.g. direct browser
		// connection), start a fresh root span so the WS session is still
		// represented in the trace.
		traceCtx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		if !trace.SpanContextFromContext(traceCtx).IsValid() {
			var span trace.Span
			traceCtx, span = otel.Tracer("openforge-ws").Start(r.Context(), "ws.session.open")
			span.End()
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("ws upgrade failed", "error", err)
			return
		}

		c := &wsConn{
			conn:             conn,
			jwtSvc:           jwtSvc,
			authRepo:         authadapter.NewPGAuthRepository(of.DB),
			userID:           claims.UserID,
			userRole:         claims.Role,
			engines:          make(map[string]*domain.QueryEngine),
			streamCancels:    make(map[string]context.CancelFunc),
			of:               of,
			pongFail:         0,
			wsRPC:            domain.NewWSRPC(conn, 30*time.Second),
			dispatchServices: wireDispatchServices(of),
			traceCtx:         traceCtx,
		}
		c.run()
	}
}

// wireDispatchServices builds the production dispatchServices from the
// OpenForge composition root. Each function pointer falls back to a
// "not configured" error if the underlying service is nil, so misconfigured
// profiles fail loudly at the WS layer instead of panicking.
func wireDispatchServices(of *profile.OpenForge) *dispatchServices {
	if of == nil {
		return nil
	}
	return &dispatchServices{
		DeactivateBranch: func(ctx context.Context, branchID string) error {
			if of.PipelineRepo == nil {
				return errors.New("pipeline repo not configured")
			}
			return of.PipelineRepo.DeactivateBranch(ctx, branchID)
		},
		ModifyScope: func(ctx context.Context, pipelineID, newRequirement string) error {
			if of.PipelineSvc == nil {
				return errors.New("pipeline service not configured")
			}
			return of.PipelineSvc.ModifyScope(ctx, pipelineID, newRequirement)
		},
		SwitchModel: func(ctx context.Context, model string) error {
			if of.LLMRouter == nil {
				return errors.New("llm router not configured")
			}
			return of.LLMRouter.SwitchModel(ctx, model)
		},
		TerminalInput: func(ctx context.Context, pipelineID, input string) error {
			if of.TerminalService == nil {
				return errors.New("terminal service not configured")
			}
			return of.TerminalService.Input(ctx, pipelineID, input)
		},
		SaveLayout: func(ctx context.Context, userID string, layout map[string]any) error {
			if of.SettingsRepo == nil {
				return errors.New("settings repo not configured")
			}
			return of.SettingsRepo.SaveLayout(ctx, userID, layout)
		},
	}
}

func verifyWebSocketRequest(r *http.Request, jwtSvc *service.JWTService) (*service.Claims, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == r.Header.Get("Authorization") {
		token = ""
	}
	if token == "" {
		token = bearerTokenFromSubprotocols(r.Header.Values("Sec-WebSocket-Protocol"))
	}
	if token == "" {
		return nil, errMissingWebSocketToken
	}
	return jwtSvc.Verify(token)
}

func bearerTokenFromSubprotocols(values []string) string {
	for _, value := range values {
		for _, protocol := range strings.Split(value, ",") {
			protocol = strings.TrimSpace(protocol)
			if strings.HasPrefix(protocol, "bearer.") {
				return strings.TrimPrefix(protocol, "bearer.")
			}
		}
	}
	return ""
}

func (c *wsConn) run() {
	defer c.conn.Close()
	defer c.cleanupEngines() // Flush all message buffers on disconnect

	c.conn.SetReadDeadline(time.Now().Add(wsPingInterval + wsPongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(wsPingInterval + wsPongTimeout))
		c.pongFail = 0
		return nil
	})

	pingTicker := time.NewTicker(wsPingInterval)
	defer pingTicker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				return
			}
			c.handleMessage(msg)
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			c.pongFail++
			if c.pongFail >= wsMaxPongFail {
				return
			}
		}
	}
}

func (c *wsConn) authenticate() bool {
	c.conn.SetReadDeadline(time.Now().Add(wsAuthTimeout))

	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		c.write(map[string]any{"type": "error", "payload": map[string]string{"message": "auth timeout"}})
		return false
	}

	var msg wsMessage
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Type != "auth" {
		c.write(map[string]any{"type": "error", "payload": map[string]string{"message": "auth required as first message"}})
		return false
	}

	var ap authPayload
	if err := json.Unmarshal(msg.Payload, &ap); err != nil || ap.Token == "" {
		c.write(map[string]any{"type": "error", "payload": map[string]string{"message": "invalid auth payload"}})
		return false
	}

	claims, err := c.jwtSvc.Verify(ap.Token)
	if err != nil {
		c.write(map[string]any{"type": "error", "payload": map[string]string{"message": "invalid token: " + err.Error()}})
		return false
	}

	c.userID = claims.UserID
	c.userRole = claims.Role
	slog.Info("ws user authenticated", "user_id", c.userID, "role", c.userRole)
	return true
}
// handleMessage parses a raw websocket payload and routes it through dispatch.
func (c *wsConn) handleMessage(raw []byte) {
	var msg wsMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	c.dispatch(msg)
}

// dispatch handles a parsed wsMessage by routing it to the correct handler.
// It is separated from handleMessage so that tests can drive the routing
// logic without needing a real *websocket.Conn.
func (c *wsConn) dispatch(msg wsMessage) {
	switch msg.Type {
	case "auth":
		// Already authenticated; re-auth ignored in Phase 2

	case "chat.send":
		var p chatSendPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return
		}

		qe := c.getOrCreateEngine(p.PipelineID, p.WorkDir)
		if qe == nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"message": "access denied: no role in project"}})
			return
		}

		// Derive stream context from c.traceCtx (extracted at WS upgrade
		// from the inbound W3C traceparent) so downstream gRPC calls to
		// the Node.js IO layer stay within the same trace.  Falls back
		// to context.Background if the connection has no trace context.
		parentCtx := c.traceCtx
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		ctx, cancel := context.WithCancel(parentCtx)
		c.mu.Lock()
		if oldCancel, ok := c.streamCancels[p.PipelineID]; ok {
			oldCancel()
		}
		c.streamCancels[p.PipelineID] = cancel
		c.mu.Unlock()

		startTime := time.Now()
		success := true

		stream, err := qe.SubmitMessage(ctx, p.Message)
		if err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"message": err.Error()}})
			if c.of.SLO != nil {
				c.of.SLO.RecordPipeline(time.Since(startTime), false)
			}
			return
		}

		for ev := range stream {
			switch ev.Type {
			case "delta":
				c.write(map[string]any{"type": "chat.stream", "payload": map[string]string{"delta": ev.Content}})
			case "tool_start":
				c.write(map[string]any{
					"type": "tool.start",
					"payload": map[string]string{
						"tool_name": ev.ToolName,
						"input":     ev.Content,
					},
				})
			case "tool_done":
				outputType := detectOutputType(ev.ToolName, ev.Content)
				c.write(map[string]any{
					"type": "tool.done",
					"payload": map[string]string{
						"tool_name":   ev.ToolName,
						"output":      ev.Content,
						"output_type": outputType,
						"status":      ev.ToolStatus,
					},
				})
			case "tool_error":
				errMsg := ""
				if ev.Error != nil {
					errMsg = ev.Error.Error()
				}
				c.write(map[string]any{
					"type": "tool.error",
					"payload": map[string]string{
						"tool_name": ev.ToolName,
						"error":     errMsg,
					},
				})
			case "context_compress":
				c.write(map[string]any{
					"type": "context.compress",
					"payload": map[string]string{
						"content": ev.Content,
					},
				})
			case "done":
				c.write(map[string]any{"type": "chat.stream_done", "payload": map[string]string{"content": ev.Content}})
			case "error":
				success = false
				errMsg := ""
				if ev.Error != nil {
					errMsg = ev.Error.Error()
				}
				c.write(map[string]any{"type": "error", "payload": map[string]string{"message": errMsg}})
			}
		}

		// Record SLO
		if c.of.SLO != nil {
			c.of.SLO.RecordPipeline(time.Since(startTime), success)
		}

		// Clean up stream cancel
		c.mu.Lock()
		delete(c.streamCancels, p.PipelineID)
		c.mu.Unlock()
		cancel()

		pipeline, err := c.of.PipelineRepo.GetByID(ctx, p.PipelineID)
		if err == nil {
			// Only emit stage_change when stage or status actually changed
			if pipeline.CurrentStage != c.lastPipelineStage || pipeline.Status != c.lastPipelineStatus {
				c.lastPipelineStage = pipeline.CurrentStage
				c.lastPipelineStatus = pipeline.Status
				c.write(map[string]any{
					"type": "pipeline.stage_change",
					"payload": map[string]string{
						"pipeline_id": pipeline.ID,
						"stage":       pipeline.CurrentStage,
						"status":      pipeline.Status,
					},
				})

				// Emit files_changed event if there are changed files
				if len(pipeline.ChangedFiles) > 0 {
					c.write(map[string]any{
						"type": "pipeline.files_changed",
						"payload": map[string]any{
							"pipeline_id":   pipeline.ID,
							"changed_files": pipeline.ChangedFiles,
						},
					})
				}
			}
		}

		// Token budget by pipeline level (with fallback to pipeline from DB)
		var budget int
		if err == nil && pipeline != nil {
			switch pipeline.Level {
			case "L1":
				budget = 4096
			case "L2":
				budget = 8192
			case "L3":
				budget = 16384
			case "L4":
				budget = 32768
			default:
				budget = 4096
			}
		} else {
			budget = 4096
		}
		used := qe.TokenUsed()
		if budget > 0 && float64(used)/float64(budget) > 0.7 {
			c.write(map[string]any{
				"type": "pipeline.token_warning",
				"payload": map[string]int{
					"used":   used,
					"budget": budget,
				},
			})
		}

	case "chat.stop":
		var p struct {
			PipelineID string `json:"pipeline_id"`
		}
		_ = json.Unmarshal(msg.Payload, &p)
		c.mu.Lock()
		if cancel, ok := c.streamCancels[p.PipelineID]; ok {
			cancel()
			delete(c.streamCancels, p.PipelineID)
		}
		c.mu.Unlock()
		c.write(map[string]any{
			"type":    "chat.stopped",
			"payload": map[string]string{"pipeline_id": p.PipelineID},
		})

	case "gate.approve":
		var gp struct {
			PipelineID string `json:"pipeline_id"`
			Stage      string `json:"stage"`
			Approver   string `json:"approver"`
			Comment    string `json:"comment"`
		}
		if err := json.Unmarshal(msg.Payload, &gp); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "invalid_payload", "message": "invalid gate.approve payload",
			}})
			return
		}
		if gp.Approver == "" {
			gp.Approver = c.userID
		}
		if c.of.GateSvc == nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "gate_approve_failed", "message": "gate service not configured",
			}})
			return
		}
		if err := c.of.GateSvc.Approve(context.Background(), gp.PipelineID, gp.Stage, gp.Approver, pipelinedomain.GateChecklist{}, gp.Comment); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "gate_approve_failed", "message": err.Error(),
			}})
			return
		}
		c.write(map[string]any{"type": "gate.notify", "payload": map[string]string{
			"pipeline_id": gp.PipelineID, "stage": gp.Stage, "event": "approved",
		}})

	case "gate.reject":
		var gp struct {
			PipelineID string `json:"pipeline_id"`
			Stage      string `json:"stage"`
			Approver   string `json:"approver"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(msg.Payload, &gp); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "invalid_payload", "message": "invalid gate.reject payload",
			}})
			return
		}
		if gp.Approver == "" {
			gp.Approver = c.userID
		}
		if c.of.GateSvc == nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "gate_reject_failed", "message": "gate service not configured",
			}})
			return
		}
		if err := c.of.GateSvc.Reject(context.Background(), gp.PipelineID, gp.Stage, gp.Approver, nil, gp.Reason); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{
				"code": "gate_reject_failed", "message": err.Error(),
			}})
			return
		}
		c.write(map[string]any{"type": "gate.notify", "payload": map[string]string{
			"pipeline_id": gp.PipelineID, "stage": gp.Stage, "event": "rejected",
		}})

	case "pipeline.cancel":
		payloadBytes, _ := json.Marshal(msg.Payload)
		var cp struct {
			PipelineID string `json:"pipeline_id"`
		}
		json.Unmarshal(payloadBytes, &cp)
		c.of.PipelineSvc.Cancel(context.Background(), cp.PipelineID)
		c.write(map[string]any{"type": "pipeline.finished", "payload": map[string]string{
			"pipeline_id": cp.PipelineID, "status": "cancelled",
		}})

	case "tool.proxy_result":
		if err := c.wsRPC.HandleProxyResult(msg.Payload); err != nil {
			slog.Error("failed to handle proxy result", "error", err)
			c.write(map[string]any{"type": "error", "payload": map[string]string{"message": err.Error()}})
		}

	case "ping":
		c.write(map[string]any{"type": "pong"})

	case "chat.edit":
		var ep struct {
			MessageID string `json:"message_id"`
			Content   string `json:"content"`
		}
		_ = json.Unmarshal(msg.Payload, &ep)
		if err := c.dispatchChatEdit(ep.MessageID, ep.Content); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "edit_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "chat.edited", "payload": map[string]any{"message_id": ep.MessageID}})

	case "chat.pause":
		c.write(map[string]any{"type": "chat.paused", "payload": map[string]any{}})

	case "chat.resume":
		c.write(map[string]any{"type": "chat.resumed", "payload": map[string]any{}})

	case "chat.retry":
		var rp struct {
			MessageID  string `json:"message_id"`
			PipelineID string `json:"pipeline_id"`
		}
		_ = json.Unmarshal(msg.Payload, &rp)
		if rp.MessageID == "" {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "retry_missing_message_id"}})
			return
		}
		if err := c.dispatchChatRetry(rp.PipelineID, rp.MessageID); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "retry_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "chat.retry_started", "payload": map[string]any{"message_id": rp.MessageID}})

	case "chat.cancel_branch":
		var bp struct {
			BranchID string `json:"branch_id"`
		}
		_ = json.Unmarshal(msg.Payload, &bp)
		if err := c.dispatchCancelBranch(bp.BranchID); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"code": "cancel_branch_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "chat.branch_cancelled", "payload": map[string]string{"branch_id": bp.BranchID}})

	case "pipeline.modify_scope":
		var sp struct {
			PipelineID     string `json:"pipeline_id"`
			NewRequirement string `json:"new_requirement"`
		}
		_ = json.Unmarshal(msg.Payload, &sp)
		if err := c.dispatchModifyScope(sp.PipelineID, sp.NewRequirement); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"code": "modify_scope_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "pipeline.scope_modified", "payload": map[string]string{"pipeline_id": sp.PipelineID}})

	case "model.switch":
		var mp struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(msg.Payload, &mp)
		if err := c.dispatchModelSwitch(mp.Model); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"code": "model_switch_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "model.switched", "payload": map[string]string{"model": mp.Model}})

	case "terminal.input":
		var tp struct {
			PipelineID string `json:"pipeline_id"`
			Input      string `json:"input"`
		}
		_ = json.Unmarshal(msg.Payload, &tp)
		if err := c.dispatchTerminalInput(tp.PipelineID, tp.Input); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "terminal_input_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "terminal.input_acked", "payload": map[string]any{}})

	case "panel.layout.save":
		var lp struct {
			UserID string         `json:"user_id"`
			Layout map[string]any `json:"layout"`
		}
		_ = json.Unmarshal(msg.Payload, &lp)
		if err := c.dispatchPanelLayoutSave(lp.UserID, lp.Layout); err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "layout_save_failed", "message": err.Error()}})
			return
		}
		c.write(map[string]any{"type": "panel.layout_saved", "payload": map[string]any{}})

	case "sync.request":
		// Client reconnect: replay any TraceEvents newer than lastSeq so
		// the UI can re-render state it missed while the socket was
		// closed. Returns one sync.replay message per missed event;
		// the absence of any replay means "you are caught up".
		var sp struct {
			PipelineID string `json:"pipeline_id"`
			LastSeq    int64  `json:"last_seq"`
		}
		_ = json.Unmarshal(msg.Payload, &sp)
		if c.of == nil || c.of.TraceStore == nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "sync_failed", "message": "trace store not configured"}})
			return
		}
		missed, err := c.of.TraceStore.ListSince(context.Background(), sp.PipelineID, sp.LastSeq)
		if err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]any{"code": "sync_failed", "message": err.Error()}})
			return
		}
		for _, ev := range missed {
			c.write(map[string]any{
				"type": "sync.replay",
				"payload": map[string]any{
					"seq":        ev.Seq,
					"event":      ev.Event,
					"payload":    ev.Payload,
					"timestamp":  ev.Timestamp,
					"pipeline_id": ev.PipelineID,
				},
			})
		}
	}
}

func (c *wsConn) getOrCreateEngine(pipelineID, workDir string) *domain.QueryEngine {
	c.mu.Lock()
	defer c.mu.Unlock()
	if qe, ok := c.engines[pipelineID]; ok {
		return qe
	}

	ctx := domain.PipelineContext{
		PipelineID:     pipelineID,
		Stage:          "impl",
		StageLevel:     "L2",
		PermissionMode: "auto",
	}

	// Try to resolve pipeline metadata from DB
	if p, err := c.of.PipelineRepo.GetByID(context.Background(), pipelineID); err == nil && p != nil {
		ctx.ProjectID = p.ProjectID

		// Multi-tenant check: verify user has a role in this project.
		if c.authRepo != nil && p.ProjectID != "" {
			role, _ := c.authRepo.GetUserRole(context.Background(), c.userID, p.ProjectID)
			if role == nil {
				// Global admin bypasses per-project role check.
				if c.userRole == "admin" || c.userRole == "superadmin" {
					slog.Debug("ws global admin bypass", "user_id", c.userID, "project_id", p.ProjectID)
				} else {
					slog.Warn("ws access denied: no role in project",
						"user_id", c.userID,
						"project_id", p.ProjectID,
						"pipeline_id", pipelineID,
					)
					return nil
				}
			}
		}

		if p.CurrentStage != "" {
			ctx.Stage = p.CurrentStage
		}
		switch p.Level {
		case "L1":
			ctx.StageLevel = "L1"
		case "L3":
			ctx.StageLevel = "L3"
		case "L4":
			ctx.StageLevel = "L4"
		default:
			ctx.StageLevel = "L2"
		}
	}

	cfg := agentport.LLMConfig{
		Provider:  c.of.Config.LLM.DefaultProvider,
		Model:     c.of.Config.LLM.DefaultModel,
		MaxTokens: 4096,
	}
	qe := domain.NewQueryEngine(c.of.LLMRouter, cfg, c.of.PromptBuilder, ctx)

	// Create tool registry with proxy executor for local file operations
	toolRegistry := domain.DefaultToolRegistryWithWorkDir(workDir)
	if c.wsRPC != nil {
		proxyExecutor := domain.NewToolProxyExecutor(c.wsRPC)
		// Override file operation tools with proxy executor
		toolRegistry["read_file"] = domain.ProxyExecutorMeta("read_file", proxyExecutor)
		toolRegistry["write_file"] = domain.ProxyExecutorMeta("write_file", proxyExecutor)
		toolRegistry["list_dir"] = domain.ProxyExecutorMeta("list_dir", proxyExecutor)
		toolRegistry["search_file"] = domain.ProxyExecutorMeta("search_file", proxyExecutor)
		toolRegistry["search_content"] = domain.ProxyExecutorMeta("search_content", proxyExecutor)
	}
	qe.SetToolRegistry(toolRegistry)
	qe.SetConversationRepo(c.of.PipelineRepo)

	// Preload conversation history from DB for reconnect resilience.
	if dbMsgs, err := c.of.PipelineRepo.GetMessages(context.Background(), pipelineID, "main"); err == nil && len(dbMsgs) > 0 {
		msgs := make([]agentport.Message, len(dbMsgs))
		for i, dm := range dbMsgs {
			msgs[i] = agentport.Message{Role: dm.Role, Content: dm.Content}
		}
		qe.LoadMessages(msgs)
	}

	c.engines[pipelineID] = qe
	return qe
}

// cleanupEngines stops flush loops for all engines to ensure buffered messages are persisted.
func (c *wsConn) cleanupEngines() {
	c.mu.Lock()
	engines := make([]*domain.QueryEngine, 0, len(c.engines))
	for _, qe := range c.engines {
		engines = append(engines, qe)
	}
	c.engines = make(map[string]*domain.QueryEngine) // Clear map
	c.mu.Unlock()

	for _, qe := range engines {
		qe.StopFlushLoop()
	}
}

func (c *wsConn) write(v any) {
	if c.writer != nil {
		c.writer(v)
		return
	}
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	c.conn.WriteJSON(v)
}

// detectOutputType determines the output type based on tool name and content.
// This helps the frontend render tool outputs appropriately (e.g., file trees for ls).
func detectOutputType(toolName, content string) string {
	switch toolName {
	case "ls", "list_dir":
		return "file_listing"
	case "bash":
		// Heuristic: if bash output looks like ls output, treat as file_listing
		if isFileListingOutput(content) {
			return "file_listing"
		}
		return "text"
	case "read_file":
		return "file_content"
	case "grep", "search_file":
		return "search_results"
	case "glob":
		return "file_list"
	default:
		return "text"
	}
}

// isFileListingOutput checks if the output looks like a file listing (ls -la format).
// Returns true if the output contains typical ls patterns like permissions, owner, size, date.
func isFileListingOutput(content string) bool {
	// Simple heuristic: check for common ls -la patterns
	// Pattern: starts with "total" or has permission strings like "drwxr-xr-x" or "-rw-r--r--"
	if len(content) < 10 {
		return false
	}
	// Check for "total" line at start (common in ls -la)
	if len(content) > 5 && content[:5] == "total" {
		return true
	}
	// Check for permission patterns (d or - followed by rwx pattern)
	lines := splitLines(content)
	if len(lines) > 0 {
		firstLine := lines[0]
		if len(firstLine) >= 10 {
			// Check for permission string pattern: starts with d or -, then rwx pattern
			if (firstLine[0] == 'd' || firstLine[0] == '-') &&
				(firstLine[1] == 'r' || firstLine[1] == '-') &&
				(firstLine[2] == 'w' || firstLine[2] == '-') &&
				(firstLine[3] == 'x' || firstLine[3] == '-') {
				return true
			}
		}
	}
	return false
}

// splitLines splits a string into lines (handles both \n and \r\n).
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
			// Handle \r\n
			if i > 0 && s[i-1] == '\r' {
				lines[len(lines)-1] = lines[len(lines)-1][:len(lines[len(lines)-1])-1]
			}
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ---------------------------------------------------------------------------
// T4 dispatch helpers — 11 new WS handler cases.
//
// Each helper is a thin wrapper around the corresponding service method. The
// wrapper exists so the production code goes through a stable seam, and so
// unit tests can stub the underlying call without spinning up a real
// websocket connection.
// ---------------------------------------------------------------------------

// dispatchChatEdit edits a previously-sent message and re-triggers the agent
// loop starting from that message. It looks up the engine for the pipeline
// the message belongs to and calls EditMessage.
func (c *wsConn) dispatchChatEdit(messageID, content string) error {
	if messageID == "" {
		return errors.New("message_id required")
	}
	qe := c.lookupEngineForMessage(messageID)
	if qe == nil {
		return errNoSuchMessage{msgID: messageID}
	}
	return qe.EditMessage(context.Background(), messageID, content)
}

// dispatchChatRetry re-sends a stream starting from the given message.
func (c *wsConn) dispatchChatRetry(pipelineID, messageID string) error {
	if messageID == "" {
		return errors.New("message_id required")
	}
	qe := c.engineFor(pipelineID)
	if qe == nil {
		return errNoSuchMessage{msgID: messageID}
	}
	return qe.ResendFromMessage(context.Background(), pipelineID, messageID)
}

// dispatchCancelBranch deactivates an active conversation branch.
func (c *wsConn) dispatchCancelBranch(branchID string) error {
	if branchID == "" {
		return errors.New("branch_id required")
	}
	if c.dispatchServices == nil || c.dispatchServices.DeactivateBranch == nil {
		return errors.New("deactivate branch not configured")
	}
	return c.dispatchServices.DeactivateBranch(context.Background(), branchID)
}

// dispatchModifyScope updates the user-visible scope of a pipeline and
// triggers a backtrack from the latest message.
func (c *wsConn) dispatchModifyScope(pipelineID, newRequirement string) error {
	if pipelineID == "" {
		return errors.New("pipeline_id required")
	}
	if c.dispatchServices == nil || c.dispatchServices.ModifyScope == nil {
		return errors.New("modify scope not configured")
	}
	return c.dispatchServices.ModifyScope(context.Background(), pipelineID, newRequirement)
}

// dispatchModelSwitch changes the active LLM model at the router level.
func (c *wsConn) dispatchModelSwitch(model string) error {
	if model == "" {
		return errors.New("model required")
	}
	if c.dispatchServices == nil || c.dispatchServices.SwitchModel == nil {
		return errors.New("switch model not configured")
	}
	return c.dispatchServices.SwitchModel(context.Background(), model)
}

// dispatchTerminalInput forwards a string of input to the in-pipeline sandbox
// terminal. The ack is intentionally fire-and-forget — terminal output is
// streamed via chat.stream events.
func (c *wsConn) dispatchTerminalInput(pipelineID, input string) error {
	if pipelineID == "" {
		return errors.New("pipeline_id required")
	}
	if c.dispatchServices == nil || c.dispatchServices.TerminalInput == nil {
		return errors.New("terminal input not configured")
	}
	return c.dispatchServices.TerminalInput(context.Background(), pipelineID, input)
}

// dispatchPanelLayoutSave persists a per-user layout configuration.
func (c *wsConn) dispatchPanelLayoutSave(userID string, layout map[string]any) error {
	if userID == "" {
		return errors.New("user_id required")
	}
	if c.dispatchServices == nil || c.dispatchServices.SaveLayout == nil {
		return errors.New("save layout not configured")
	}
	return c.dispatchServices.SaveLayout(context.Background(), userID, layout)
}

// engineFor returns the engine registered for a pipeline, or nil.
func (c *wsConn) engineFor(pipelineID string) *domain.QueryEngine {
	if pipelineID == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.engines[pipelineID]
}

// lookupEngineForMessage is a best-effort lookup. In Phase 2 we don't index
// messages back to pipelines, so we walk the engine map. The first engine
// whose conversation repo can find the message wins. Returns nil when no
// engine claims the message — the caller surfaces errNoSuchMessage.
func (c *wsConn) lookupEngineForMessage(messageID string) *domain.QueryEngine {
	c.mu.Lock()
	engines := make([]*domain.QueryEngine, 0, len(c.engines))
	for _, qe := range c.engines {
		engines = append(engines, qe)
	}
	c.mu.Unlock()
	if len(engines) == 0 {
		return nil
	}
	if len(engines) == 1 {
		return engines[0]
	}
	for _, qe := range engines {
		if qe.HasMessage(context.Background(), messageID) {
			return qe
		}
	}
	return engines[0]
}

// errNoSuchMessage is returned by dispatchChatEdit / dispatchChatRetry when
// the target message_id is not present in any engine's history. The string
// form is surfaced to WS clients as the "message" field of the error payload.
type errNoSuchMessage struct{ msgID string }

func (e errNoSuchMessage) Error() string { return "no such message: " + e.msgID }
