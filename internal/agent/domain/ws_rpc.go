package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSRPC manages WebSocket RPC request/response matching.
// It maintains a map of pending requests and their response channels.
type WSRPC struct {
	mu       sync.RWMutex
	pending  map[string]chan ToolProxyResult
	conn     *websocket.Conn
	timeout  time.Duration
}

// NewWSRPC creates a new WSRPC instance with the given WebSocket connection and timeout.
func NewWSRPC(conn *websocket.Conn, timeout time.Duration) *WSRPC {
	return &WSRPC{
		pending: make(map[string]chan ToolProxyResult),
		conn:    conn,
		timeout: timeout,
	}
}

// SendProxyRequest sends a tool proxy request via WebSocket and waits for the response.
// It returns the result or an error if the request times out or fails.
func (r *WSRPC) SendProxyRequest(ctx context.Context, req ToolProxyRequest) (ToolProxyResult, error) {
	// Create response channel
	ch := make(chan ToolProxyResult, 1)
	
	r.mu.Lock()
	r.pending[req.RequestID] = ch
	r.mu.Unlock()
	
	// Clean up on exit
	defer func() {
		r.mu.Lock()
		delete(r.pending, req.RequestID)
		r.mu.Unlock()
	}()
	
	// Send request via WebSocket
	msg := map[string]any{
		"type":    "tool.proxy_request",
		"payload": req,
	}
	
	if err := r.conn.WriteJSON(msg); err != nil {
		return ToolProxyResult{}, fmt.Errorf("failed to send proxy request: %w", err)
	}
	
	// Wait for response with timeout
	select {
	case result := <-ch:
		return result, nil
	case <-time.After(r.timeout):
		return ToolProxyResult{}, fmt.Errorf("proxy request timed out after %v", r.timeout)
	case <-ctx.Done():
		return ToolProxyResult{}, ctx.Err()
	}
}

// HandleProxyResult handles an incoming proxy result message from the WebSocket.
// It finds the corresponding pending request and sends the result to its channel.
func (r *WSRPC) HandleProxyResult(msg []byte) error {
	var result ToolProxyResult
	if err := json.Unmarshal(msg, &result); err != nil {
		return fmt.Errorf("failed to unmarshal proxy result: %w", err)
	}
	
	r.mu.RLock()
	ch, exists := r.pending[result.RequestID]
	r.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no pending request found for ID: %s", result.RequestID)
	}
	
	// Send result to waiting goroutine
	select {
	case ch <- result:
		return nil
	default:
		return fmt.Errorf("failed to send result to channel for ID: %s", result.RequestID)
	}
}

// Close cleans up all pending requests.
func (r *WSRPC) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for id, ch := range r.pending {
		close(ch)
		delete(r.pending, id)
	}
}