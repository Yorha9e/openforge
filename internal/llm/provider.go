package llm

import "context"

// ChatRequest is a normalized request for all providers.
type ChatRequest struct {
	Model        string
	Messages     []Message
	MaxTokens    int
	SystemPrompt string
}

// Message is a normalized chat message.
type Message struct {
	Role    string // "user" | "assistant" | "system"
	Content string
}

// ChatResponse is a normalized response.
type ChatResponse struct {
	Content    string
	StopReason string
	Usage      Usage
}

// Usage holds token counts.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// StreamChunk represents a single streaming delta.
type StreamChunk struct {
	Delta      string
	StopReason string
	Usage      *Usage // message_delta events carry usage
}

// Provider abstracts an LLM provider backend.
type Provider interface {
	// Chat sends a request and returns the full response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream sends a request and streams response chunks.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}
