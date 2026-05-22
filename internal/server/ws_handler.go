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
)

type wsMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type chatSendPayload struct {
	PipelineID string `json:"pipeline_id"`
	Message    string `json:"message"`
}

func handleChatWS(of *profile.OpenForge, jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		if userID == "" {
			writeError(w, 401, "authentication required")
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade failed: %v", err)
			return
		}

		c := &wsConn{
			conn:     conn,
			userID:   userID,
			engines:  make(map[string]*domain.QueryEngine),
			of:       of,
			pongFail: 0,
		}
		c.run()
	}
}

type wsConn struct {
	conn     *websocket.Conn
	userID   string
	mu       sync.Mutex
	engines  map[string]*domain.QueryEngine
	of       *profile.OpenForge
	pongFail int
}

func (c *wsConn) run() {
	defer c.conn.Close()

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

func (c *wsConn) handleMessage(raw []byte) {
	var msg wsMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "chat.send":
		payloadBytes, _ := json.Marshal(msg.Payload)
		var p chatSendPayload
		if err := json.Unmarshal(payloadBytes, &p); err != nil {
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

	case "chat.stop":
		// Phase 3: cancel active stream

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
	cfg := agentport.LLMConfig{
		Provider:  c.of.Config.LLM.DefaultProvider,
		Model:     c.of.Config.LLM.DefaultModel,
		MaxTokens: 4096,
	}
	qe := domain.NewQueryEngine(c.of.LLMRouter, cfg)
	c.engines[pipelineID] = qe
	return qe
}

func (c *wsConn) write(v any) {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	c.conn.WriteJSON(v)
}
