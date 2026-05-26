package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/gen/go/agent/v1/agentv1connect"
	"openforge/internal/agent/port"

	"connectrpc.com/connect"
)

// LLMClient implements port.LLMRouterClient via ConnectRPC to the Node.js LLM Router.
type LLMClient struct {
	httpClient *http.Client
	client     agentv1connect.LLMRouterServiceClient
}

// NewLLMClient creates a ConnectRPC client targeting addr (e.g. "http://127.0.0.1:50051").
func NewLLMClient(addr string) (*LLMClient, error) {
	baseURL := fmt.Sprintf("http://%s", addr)
	httpClient := &http.Client{}
	return &LLMClient{
		httpClient: httpClient,
		client:     agentv1connect.NewLLMRouterServiceClient(httpClient, baseURL),
	}, nil
}

// Chat sends a unary Chat request and returns the aggregated response.
func (c *LLMClient) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	pbReq := toProtoRequest(req)
	connectReq := connect.NewRequest(pbReq)
	pbResp, err := c.client.Chat(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect chat: %w", err)
	}
	return fromProtoResponse(pbResp.Msg), nil
}

// ChatStream sends a server-streaming Chat request and returns a channel of deltas.
func (c *LLMClient) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan port.StreamChunk, error) {
	pbReq := toProtoRequest(req)
	connectReq := connect.NewRequest(pbReq)
	stream, err := c.client.ChatStream(ctx, connectReq)
	if err != nil {
		return nil, fmt.Errorf("connect chat stream: %w", err)
	}
	ch := make(chan port.StreamChunk, 64)
	go func() {
		defer close(ch)
		for stream.Receive() {
			msg := stream.Msg()
			if msg.Delta != nil && msg.Delta.Text != nil {
				ch <- port.StreamChunk{Delta: *msg.Delta.Text}
			}
		}
	}()
	return ch, nil
}

// Close releases idle HTTP connections.
func (c *LLMClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// ---------------------------------------------------------------------------
// conversion helpers
// ---------------------------------------------------------------------------

func toProtoRequest(req port.ChatRequest) *agentv1.LLMChatRequest {
	pbMessages := make([]*agentv1.LLMMessage, len(req.Messages))
	for i, m := range req.Messages {
		text := m.Content
		pbMessages[i] = &agentv1.LLMMessage{
			Role: m.Role,
			Content: []*agentv1.LLMContentBlock{
				{Type: "text", Text: &text},
			},
		}
	}

	temp := req.Config.Temperature
	maxTok := int32(req.Config.MaxTokens)

	var systemPrompt *string
	if req.SystemPrompt != "" {
		systemPrompt = &req.SystemPrompt
	}

	// Serialize tools for the proto request
	pbTools := make([]*agentv1.LLMTool, len(req.Tools))
	for i, t := range req.Tools {
		schemaBytes, _ := json.Marshal(t.InputSchema)
		pbTools[i] = &agentv1.LLMTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaBytes,
		}
	}

	return &agentv1.LLMChatRequest{
		PipelineId:   "cli-session",
		Messages:     pbMessages,
		SystemPrompt: systemPrompt,
		Tools:        pbTools,
		Config: &agentv1.LLMConfig{
			Provider:    req.Config.Provider,
			Model:       req.Config.Model,
			ApiEndpoint: req.Config.APIEndpoint,
			ApiKey:      req.Config.APIKey,
			Temperature: &temp,
			MaxTokens:   &maxTok,
		},
	}
}

func fromProtoResponse(pbResp *agentv1.LLMChatResponse) *port.ChatResponse {
	resp := &port.ChatResponse{ID: pbResp.Id}
	for _, block := range pbResp.Content {
		if block.Text != nil {
			resp.Content += *block.Text
		}
	}
	resp.StopReason = pbResp.StopReason
	if pbResp.Usage != nil {
		resp.Usage = &port.Usage{
			InputTokens:  pbResp.Usage.InputTokens,
			OutputTokens: pbResp.Usage.OutputTokens,
		}
	}
	return resp
}
