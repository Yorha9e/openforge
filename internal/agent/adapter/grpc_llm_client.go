package adapter

import (
	"context"
	"fmt"
	"io"

	agentv1 "openforge/gen/go/agent/v1"
	"openforge/internal/agent/port"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LLMClient implements port.LLMRouterClient via gRPC to the Node.js LLM Router.
type LLMClient struct {
	conn   *grpc.ClientConn
	client agentv1.LLMRouterServiceClient
}

// NewLLMClient dials addr (e.g. "localhost:50051") and returns a ready client.
func NewLLMClient(addr string) (*LLMClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &LLMClient{
		conn:   conn,
		client: agentv1.NewLLMRouterServiceClient(conn),
	}, nil
}

// Chat sends a unary Chat request and returns the aggregated response.
func (c *LLMClient) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	pbReq := toProtoRequest(req)
	pbResp, err := c.client.Chat(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("grpc chat: %w", err)
	}
	return fromProtoResponse(pbResp), nil
}

// ChatStream sends a streaming Chat request and returns a channel of text deltas.
func (c *LLMClient) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan string, error) {
	pbReq := toProtoRequest(req)
	stream, err := c.client.ChatStream(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("grpc chat stream: %w", err)
	}
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			if msg.Delta != nil && msg.Delta.Text != nil {
				ch <- *msg.Delta.Text
			}
		}
	}()
	return ch, nil
}

// Close tears down the underlying gRPC connection.
func (c *LLMClient) Close() error {
	return c.conn.Close()
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

	return &agentv1.LLMChatRequest{
		PipelineId: "cli-session",
		Messages:   pbMessages,
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
	if pbResp.Usage != nil {
		resp.Usage = port.Usage{
			InputTokens:  pbResp.Usage.InputTokens,
			OutputTokens: pbResp.Usage.OutputTokens,
		}
	}
	return resp
}
