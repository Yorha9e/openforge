package domain

import (
	"context"
	"testing"

	agentport "openforge/internal/agent/port"
)

type stubSummaryClient struct {
	summary string
	err     error
}

func (s *stubSummaryClient) Chat(ctx context.Context, req agentport.ChatRequest) (*agentport.ChatResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &agentport.ChatResponse{Content: s.summary, StopReason: "end_turn"}, nil
}

func (s *stubSummaryClient) ChatStream(ctx context.Context, req agentport.ChatRequest) (<-chan agentport.StreamChunk, error) {
	ch := make(chan agentport.StreamChunk, 1)
	ch <- agentport.StreamChunk{Delta: s.summary, FinishReason: "end_turn"}
	close(ch)
	return ch, nil
}

func TestCompressor_NeedsCompression(t *testing.T) {
	comp := NewCompressor(nil, DefaultCompressionConfig())

	// 80% of 200000 = 160000
	if comp.NeedsCompression(170000, 200000) {
		t.Log("170K > 160K → needs compression")
	} else {
		t.Error("expected NeedsCompression=true")
	}

	if comp.NeedsCompression(100000, 200000) {
		t.Error("expected NeedsCompression=false for 100K/200K")
	}
}

func TestCompressor_Compress_Normal(t *testing.T) {
	client := &stubSummaryClient{summary: "Compressed: user asked about auth, implemented JWT, fixed 2 bugs."}
	comp := NewCompressor(client, DefaultCompressionConfig())

	// 30 messages → keep last 20 (10 rounds × 2), compress first 10
	messages := make([]agentport.Message, 30)
	for i := 0; i < 30; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = agentport.Message{Role: role, Content: "message content"}
	}

	result, err := comp.Compress(context.Background(), messages, 170000, 200000, false)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	// Should have summary + keepLast messages
	if result[0].Role != "user" {
		t.Errorf("expected first message to be summary (role=user), got role=%s", result[0].Role)
	}
	if !containsString(result[0].Content, "Compressed:") {
		t.Errorf("summary message missing expected content: %s", result[0].Content)
	}
}

func TestCompressor_Compress_Aggressive(t *testing.T) {
	client := &stubSummaryClient{summary: "Aggressive summary."}
	comp := NewCompressor(client, DefaultCompressionConfig())

	messages := make([]agentport.Message, 60)
	for i := range messages {
		messages[i] = agentport.Message{Role: "user", Content: "msg"}
	}

	result, err := comp.Compress(context.Background(), messages, 190000, 200000, true)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	// Aggressive: keep only 5 rounds (10 msgs)
	expectedKeep := 10
	if len(result)-1 < expectedKeep {
		t.Errorf("expected at least %d keep messages + 1 summary, got %d total", expectedKeep, len(result))
	}
}

func TestCompressor_SummaryFallback(t *testing.T) {
	// Haiku unavailable → falls back to text truncation
	client := &stubSummaryClient{err: context.DeadlineExceeded}
	comp := NewCompressor(client, DefaultCompressionConfig())

	messages := make([]agentport.Message, 30)
	for i := range messages {
		messages[i] = agentport.Message{Role: "user", Content: "test message"}
	}

	result, err := comp.Compress(context.Background(), messages, 170000, 200000, false)
	if err != nil {
		t.Fatalf("Compress should not error on fallback: %v", err)
	}
	if !containsString(result[0].Content, "(truncated)") {
		t.Error("expected text truncation fallback when Haiku unavailable")
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	messages := []agentport.Message{
		{Role: "user", Content: "add login endpoint"},
		{Role: "assistant", Content: "I'll create the route at backend/src/routes/auth.ts"},
	}

	prompt := buildSummaryPrompt(messages)
	if !containsString(prompt, "Key decisions") {
		t.Error("missing 5-point preservation format")
	}
	if !containsString(prompt, "login endpoint") {
		t.Error("missing user message content")
	}
}

func TestEstimateTokens(t *testing.T) {
	messages := []agentport.Message{
		{Role: "user", Content: "hello world"},
		{Role: "assistant", Content: "hi there how are you"},
	}
	tokens := EstimateTokens(messages)
	if tokens < 5 || tokens > 10 {
		t.Errorf("expected ~7 tokens, got %d", tokens)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
