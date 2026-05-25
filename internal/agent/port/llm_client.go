package port

import "context"

type ChatRequest struct {
	Messages      []Message
	SystemPrompt  string
	Config        LLMConfig
}

type Message struct {
	Role    string
	Content string
}

type LLMConfig struct {
	Provider    string
	Model       string
	APIEndpoint string
	APIKey      string
	MaxTokens   int
	Temperature float64
}

type ChatResponse struct {
	ID           string
	Content      string
	StopReason   string // "end_turn" | "tool_use" | "max_tokens" | "stop_sequence"
	Usage        *Usage
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

type StreamChunk struct {
	Delta        string
	FinishReason string // final chunk carries stop_reason
	Usage        *Usage // final chunk may carry usage
}

type LLMRouterClient interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}
