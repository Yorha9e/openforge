package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"openforge/internal/agent/domain"
	agentport "openforge/internal/agent/port"
	authadapter "openforge/internal/auth/adapter"
	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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

type wsMessage struct {
	Type    string `json:"type"`
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
	conn     *websocket.Conn
	jwtSvc   *service.JWTService
	authRepo *authadapter.PGAuthRepository
	userID   string
	userRole string
	mu       sync.Mutex
	engines  map[string]*domain.QueryEngine
	of       *profile.OpenForge
	pongFail int
}

func handleChatWS(of *profile.OpenForge, jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("ws upgrade failed", "error", err)
			return
		}

		c := &wsConn{
			conn:     conn,
			jwtSvc:   jwtSvc,
			authRepo: authadapter.NewPGAuthRepository(of.DB),
			engines:  make(map[string]*domain.QueryEngine),
			of:       of,
			pongFail: 0,
		}
		c.run()
	}
}

func (c *wsConn) run() {
	defer c.conn.Close()

	// First-frame auth with timeout
	if !c.authenticate() {
		return
	}

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

func (c *wsConn) handleMessage(raw []byte) {
	var msg wsMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

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

		ctx := context.Background()

		stream, err := qe.SubmitMessage(ctx, p.Message)
		if err != nil {
			c.write(map[string]any{"type": "error", "payload": map[string]string{"message": err.Error()}})
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
				errMsg := ""
				if ev.Error != nil {
					errMsg = ev.Error.Error()
				}
				c.write(map[string]any{"type": "error", "payload": map[string]string{"message": errMsg}})
			}
		}

		pipeline, err := c.of.PipelineRepo.GetByID(ctx, p.PipelineID)
		if err == nil {
			c.write(map[string]any{
				"type": "pipeline.stage_change",
				"payload": map[string]string{
					"pipeline_id": pipeline.ID,
					"stage":       pipeline.CurrentStage,
					"status":      pipeline.Status,
				},
			})
		}

		if qe.TokenUsed() > 3200 {
			c.write(map[string]any{
				"type": "pipeline.token_warning",
				"payload": map[string]int{
					"used":   qe.TokenUsed(),
					"budget": 4096,
				},
			})
		}

	case "chat.stop":
		// Phase 3: cancel active stream

	case "gate.approve":
		payloadBytes, _ := json.Marshal(msg.Payload)
		var gp struct {
			PipelineID string `json:"pipeline_id"`
			Stage      string `json:"stage"`
		}
		json.Unmarshal(payloadBytes, &gp)
		c.write(map[string]any{"type": "gate.notify", "payload": map[string]string{
			"pipeline_id": gp.PipelineID, "stage": gp.Stage, "event": "approved",
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

	case "ping":
		c.write(map[string]any{"type": "pong"})
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
		qe.SetToolRegistry(domain.DefaultToolRegistryWithWorkDir(workDir))
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

func (c *wsConn) write(v any) {
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
