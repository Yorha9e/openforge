package domain

import (
	"context"
	"fmt"
	"sync"

	"openforge/internal/agent/port"
	"openforge/internal/agent/service"
)

type AgentCoordinator struct {
	mu        sync.Mutex
	agents    map[string]*AgentInstance
	llmClient port.LLMRouterClient
	channels  map[string]*service.CSPChannel
}

type AgentInstance struct {
	ID         string
	PipelineID string
	Role       string
	Channel    *service.CSPChannel
}

func NewCoordinator(llmClient port.LLMRouterClient) *AgentCoordinator {
	return &AgentCoordinator{
		agents:    make(map[string]*AgentInstance),
		llmClient: llmClient,
		channels:  make(map[string]*service.CSPChannel),
	}
}

func (c *AgentCoordinator) Chat(ctx context.Context, messages []port.Message, config port.LLMConfig) (string, error) {
	resp, err := c.llmClient.Chat(ctx, port.ChatRequest{
		Messages: messages,
		Config:   config,
	})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}
	return resp.Content, nil
}

func (c *AgentCoordinator) ChatStream(ctx context.Context, messages []port.Message, config port.LLMConfig) (<-chan string, error) {
	return c.llmClient.ChatStream(ctx, port.ChatRequest{
		Messages: messages,
		Config:   config,
	})
}
