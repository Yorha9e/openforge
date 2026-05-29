package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"empty", "", false},
		{"workspace path", "/workspace/main.go", false},
		{"workspace root", "/workspace/", false},
		{"unix absolute", "/home/user/file.go", true},
		{"windows absolute", "C:\\Users\\user\\file.go", true},
		{"relative", "file.go", true},
		{"relative with dir", "src/main.go", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLocalPath(tt.path); got != tt.want {
				t.Errorf("IsLocalPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestWSRPC_SendProxyRequest_Timeout(t *testing.T) {
	// Create a test WebSocket server that doesn't respond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Keep connection open but don't respond
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()
	
	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Create WSRPC with short timeout
	rpc := NewWSRPC(conn, 100*time.Millisecond)
	
	// Send request (should timeout)
	ctx := context.Background()
	req := ToolProxyRequest{
		RequestID: "test-1",
		Tool:      "read_file",
		Params:    map[string]any{"path": "/test/file.go"},
	}
	
	_, err = rpc.SendProxyRequest(ctx, req)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestWSRPC_HandleProxyResult(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read messages and respond
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			
			// Parse the request
			var wsMsg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}
			
			if wsMsg.Type == "tool.proxy_request" {
				var req ToolProxyRequest
				if err := json.Unmarshal(wsMsg.Payload, &req); err != nil {
					continue
				}
				
				// Send response
				result := ToolProxyResult{
					RequestID: req.RequestID,
					Success:   true,
					Data:      map[string]any{"output": "file content"},
				}
				
				resp := map[string]any{
					"type":    "tool.proxy_result",
					"payload": result,
				}
				conn.WriteJSON(resp)
			}
		}
	}))
	defer server.Close()
	
	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Create WSRPC
	rpc := NewWSRPC(conn, 5*time.Second)
	
	// Start reading responses in background
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			
			var wsMsg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}
			
			if wsMsg.Type == "tool.proxy_result" {
				rpc.HandleProxyResult(wsMsg.Payload)
			}
		}
	}()
	
	// Send request
	ctx := context.Background()
	req := ToolProxyRequest{
		RequestID: "test-2",
		Tool:      "read_file",
		Params:    map[string]any{"path": "/test/file.go"},
	}
	
	result, err := rpc.SendProxyRequest(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	
	if !result.Success {
		t.Error("expected success, got failure")
	}
	
	if result.Data["output"] != "file content" {
		t.Errorf("expected 'file content', got %v", result.Data["output"])
	}
}

func TestToolProxyExecutor_Execute(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read messages and respond
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			
			// Parse the request
			var wsMsg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}
			
			if wsMsg.Type == "tool.proxy_request" {
				var req ToolProxyRequest
				if err := json.Unmarshal(wsMsg.Payload, &req); err != nil {
					continue
				}
				
				// Send response
				result := ToolProxyResult{
					RequestID: req.RequestID,
					Success:   true,
					Data:      map[string]any{"output": "file content from client"},
				}
				
				resp := map[string]any{
					"type":    "tool.proxy_result",
					"payload": result,
				}
				conn.WriteJSON(resp)
			}
		}
	}))
	defer server.Close()
	
	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Create WSRPC and executor
	rpc := NewWSRPC(conn, 5*time.Second)
	executor := NewToolProxyExecutor(rpc)
	
	// Start reading responses in background
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			
			var wsMsg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				continue
			}
			
			if wsMsg.Type == "tool.proxy_result" {
				rpc.HandleProxyResult(wsMsg.Payload)
			}
		}
	}()
	
	// Test with local path (should proxy)
	ctx := context.Background()
	args := map[string]interface{}{
		"tool": "read_file",
		"path": "C:\\Users\\test\\file.go",
	}
	
	output, err := executor.Execute(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	
	if output != "file content from client" {
		t.Errorf("expected 'file content from client', got %q", output)
	}
	
	// Test with workspace path (should not proxy)
	args["path"] = "/workspace/main.go"
	_, err = executor.Execute(ctx, args)
	if err == nil {
		t.Error("expected error for workspace path, got nil")
	}
}

func TestProxyExecutorMeta(t *testing.T) {
	// Create a dummy executor
	executor := &ToolProxyExecutor{}
	
	// Test meta creation
	meta := ProxyExecutorMeta("read_file", executor)
	
	if meta.Name != "read_file" {
		t.Errorf("expected name 'read_file', got %q", meta.Name)
	}
	
	if !meta.IsConcurrencySafe {
		t.Error("expected concurrency safe to be true")
	}
	
	if meta.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", meta.Timeout)
	}
	
	if meta.Executor == nil {
		t.Error("expected executor to be set")
	}
}