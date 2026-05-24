package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"openforge/internal/agent/domain"
	agentport "openforge/internal/agent/port"
	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" || origin == ""
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
}

type authPayload struct {
	Token string `json:"token"`
}

type wsConn struct {
	conn     *websocket.Conn
	jwtSvc   *service.JWTService
	userID   string
	mu       sync.Mutex
	engines  map[string]*domain.QueryEngine
	of       *profile.OpenForge
	pongFail int
}

func handleChatWS(of *profile.OpenForge, jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade failed: %v", err)
			return
		}

		c := &wsConn{
			conn:     conn,
			jwtSvc:   jwtSvc,
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
	log.Printf("ws: user %s authenticated", c.userID)
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

		qe := c.getOrCreateEngine(p.PipelineID)
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

func (c *wsConn) getOrCreateEngine(pipelineID string) *domain.QueryEngine {
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
	c.engines[pipelineID] = qe
	return qe
}

func (c *wsConn) write(v any) {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	c.conn.WriteJSON(v)
}
