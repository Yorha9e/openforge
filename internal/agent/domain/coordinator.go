package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"openforge/internal/agent/port"
	"openforge/internal/agent/service"
)

const maxAgents = 5

type AgentCoordinator struct {
	mu        sync.Mutex
	agents    map[string]*AgentInstance
	llmClient port.LLMRouterClient
	toolReg   port.ToolRegistryClientFull
	channels  map[string]*service.CSPChannel
}

type AgentInstance struct {
	ID         string
	PipelineID string
	Role       string
	ParentID   string
	Channel    *service.CSPChannel
	CreatedAt  time.Time
}

func NewCoordinator(llmClient port.LLMRouterClient, toolReg port.ToolRegistryClientFull) *AgentCoordinator {
	return &AgentCoordinator{
		agents:    make(map[string]*AgentInstance),
		llmClient: llmClient,
		toolReg:   toolReg,
		channels:  make(map[string]*service.CSPChannel),
	}
}

// Spawn creates a new agent and its CSP channel.
func (c *AgentCoordinator) Spawn(ctx context.Context, id, pipelineID, role, parentID string) (*AgentInstance, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.agents) >= maxAgents {
		return nil, fmt.Errorf("max agents (%d) reached", maxAgents)
	}

	ch := service.NewCSPChannel(id, 64)
	agent := &AgentInstance{
		ID:         id,
		PipelineID: pipelineID,
		Role:       role,
		ParentID:   parentID,
		Channel:    ch,
		CreatedAt:  time.Now(),
	}
	c.agents[id] = agent
	c.channels[id] = ch
	return agent, nil
}

// Delegate sends a message from one agent to another.
func (c *AgentCoordinator) Delegate(ctx context.Context, fromID, toID string, msg service.Message) error {
	c.mu.Lock()
	target, ok := c.agents[toID]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("agent %q not found", toID)
	}
	return target.Channel.Send(ctx, msg)
}

// Broadcast sends a message to all agents except the sender.
func (c *AgentCoordinator) Broadcast(ctx context.Context, fromID string, msg service.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, agent := range c.agents {
		if id == fromID {
			continue
		}
		if err := agent.Channel.Send(ctx, msg); err != nil {
			return fmt.Errorf("broadcast to %q: %w", id, err)
		}
	}
	return nil
}

// AgentCount returns the number of spawned agents.
func (c *AgentCoordinator) AgentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.agents)
}

// ListAgents returns all agent instances.
func (c *AgentCoordinator) ListAgents() []AgentInstance {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]AgentInstance, 0, len(c.agents))
	for _, a := range c.agents {
		result = append(result, *a)
	}
	return result
}

// Terminate removes an agent and closes its channel.
func (c *AgentCoordinator) Terminate(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.agents[id]
	if !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	delete(c.agents, id)
	delete(c.channels, id)
	return nil
}

func (c *AgentCoordinator) Chat(ctx context.Context, messages []port.Message, config port.LLMConfig) (string, error) {
	resp, err := c.llmClient.Chat(ctx, port.ChatRequest{Messages: messages, Config: config})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}
	return resp.Content, nil
}

func (c *AgentCoordinator) ChatStream(ctx context.Context, messages []port.Message, config port.LLMConfig) (<-chan port.StreamChunk, error) {
	return c.llmClient.ChatStream(ctx, port.ChatRequest{Messages: messages, Config: config})
}

// SearchTools delegates to ToolRegistry for tool discovery.
func (c *AgentCoordinator) SearchTools(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	if c.toolReg == nil {
		return nil, fmt.Errorf("tool registry not available")
	}
	return c.toolReg.Search(ctx, query, topK)
}

// RunTool executes a tool by name.
func (c *AgentCoordinator) RunTool(ctx context.Context, call port.ToolCall) (port.ToolResult, error) {
	if c.toolReg == nil {
		return port.ToolResult{}, fmt.Errorf("tool registry not available")
	}
	return c.toolReg.Run(ctx, call)
}
