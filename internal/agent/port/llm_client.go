package port

import "context"

type ChatRequest struct {
	Messages []Message
	Config   LLMConfig
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
	ID      string
	Content string
	Usage   Usage
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

type LLMRouterClient interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan string, error)
}
