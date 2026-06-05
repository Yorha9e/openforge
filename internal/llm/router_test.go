package llm

import (
	"context"
	"errors"
	"testing"

	"openforge/internal/agent/port"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	chatResp  ChatResponse
	chatErr   error
	streamCh  <-chan StreamChunk
	streamErr error
}

func (m *mockProvider) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return m.chatResp, m.chatErr
}

func (m *mockProvider) ChatStream(_ context.Context, _ ChatRequest) (<-chan StreamChunk, error) {
	return m.streamCh, m.streamErr
}

func newTestRouter() (*Router, *Registry) {
	reg := NewRegistry()
	router := NewRouter(reg, nil)
	return router, reg
}

func TestRouterChatSuccess(t *testing.T) {
	router, _ := newTestRouter()
	provider := &mockProvider{chatResp: ChatResponse{Content: "hello", StopReason: "end_turn"}}
	router.RegisterProvider("anthropic", provider)

	resp, err := router.Chat(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("got %q, want %q", resp.Content, "hello")
	}
}

func TestRouterChatFallback(t *testing.T) {
	router, _ := newTestRouter()
	primary := &mockProvider{chatErr: errors.New("primary down")}
	fallback := &mockProvider{chatResp: ChatResponse{Content: "fallback ok"}}
	router.RegisterProvider("anthropic", primary)
	router.RegisterProvider("deepseek", fallback)

	resp, err := router.Chat(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("fallback should succeed: %v", err)
	}
	if resp.Content != "fallback ok" {
		t.Fatalf("got %q from fallback", resp.Content)
	}
}

func TestRouterChatAllFail(t *testing.T) {
	router, _ := newTestRouter()
	primary := &mockProvider{chatErr: errors.New("primary down")}
	fallback := &mockProvider{chatErr: errors.New("fallback down")}
	router.RegisterProvider("anthropic", primary)
	router.RegisterProvider("deepseek", fallback)

	_, err := router.Chat(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestRouterChatStreamSuccess(t *testing.T) {
	router, _ := newTestRouter()
	srcCh := make(chan StreamChunk, 2)
	srcCh <- StreamChunk{Delta: "hello"}
	srcCh <- StreamChunk{Delta: " world", StopReason: "end_turn"}
	close(srcCh)
	router.RegisterProvider("anthropic", &mockProvider{streamCh: srcCh})

	ch, err := router.ChatStream(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	for chunk := range ch {
		got += chunk.Delta
	}
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestRouterChatStreamFallback(t *testing.T) {
	router, _ := newTestRouter()
	primary := &mockProvider{streamErr: errors.New("stream down")}
	srcCh := make(chan StreamChunk, 1)
	srcCh <- StreamChunk{Delta: "fb", StopReason: "end_turn"}
	close(srcCh)
	fallback := &mockProvider{streamCh: srcCh}
	router.RegisterProvider("anthropic", primary)
	router.RegisterProvider("deepseek", fallback)

	ch, err := router.ChatStream(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream fallback should succeed: %v", err)
	}
	var got string
	for chunk := range ch {
		got += chunk.Delta
	}
	if got != "fb" {
		t.Fatalf("got %q, want %q", got, "fb")
	}
}

func TestRouterChatStreamAllFail(t *testing.T) {
	router, _ := newTestRouter()
	primary := &mockProvider{streamErr: errors.New("primary stream down")}
	fallback := &mockProvider{streamErr: errors.New("fallback stream down")}
	router.RegisterProvider("anthropic", primary)
	router.RegisterProvider("deepseek", fallback)

	_, err := router.ChatStream(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error when all stream providers fail")
	}
}

func TestRouterChatUnknownModel(t *testing.T) {
	router, _ := newTestRouter()

	_, err := router.Chat(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "nonexistent", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestRouterChatStreamUnknownModel(t *testing.T) {
	router, _ := newTestRouter()

	_, err := router.ChatStream(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "nonexistent", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestRouterChatNoProviderRegistered(t *testing.T) {
	router, _ := newTestRouter()
	// Don't register any providers

	_, err := router.Chat(context.Background(), port.ChatRequest{
		Config:   port.LLMConfig{Model: "sonnet", MaxTokens: 1024},
		Messages: []port.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error when no provider registered")
	}
}

func TestRouterListModels(t *testing.T) {
	router, _ := newTestRouter()
	models := router.ListModels()
	if len(models) == 0 {
		t.Fatal("expected seeded models")
	}
	found := false
	for _, m := range models {
		if m.Alias == "sonnet" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'sonnet' in model list")
	}
}
