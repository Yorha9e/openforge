package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider implements Provider for the Anthropic Messages API.
type AnthropicProvider struct {
	BaseURL string
	APIKey  string
	client  *http.Client
}

func NewAnthropicProvider(baseURL, apiKey string) *AnthropicProvider {
	return &AnthropicProvider{BaseURL: baseURL, APIKey: apiKey, client: &http.Client{}}
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := p.buildRequestBody(req, false)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("anthropic: %d %s", resp.StatusCode, string(b))
	}
	return p.parseResponse(resp.Body)
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	body := p.buildRequestBody(req, true)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic stream: %d %s", resp.StatusCode, string(b))
	}

	ch := make(chan StreamChunk, 64)
	go p.readSSE(resp.Body, ch)
	return ch, nil
}

func (p *AnthropicProvider) buildRequestBody(req ChatRequest, stream bool) []byte {
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	payload := map[string]interface{}{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"messages":   messages,
		"stream":     stream,
	}
	if req.SystemPrompt != "" {
		payload["system"] = req.SystemPrompt
	}
	b, _ := json.Marshal(payload)
	return b
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

func (p *AnthropicProvider) parseResponse(r io.Reader) (ChatResponse, error) {
	var result struct {
		Content    []struct{ Text string }
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
	}
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return ChatResponse{}, err
	}
	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}
	return ChatResponse{
		Content:    text,
		StopReason: result.StopReason,
		Usage:      Usage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens},
	}, nil
}

func (p *AnthropicProvider) readSSE(r io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		// Parse event type first
		var eventType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &eventType); err != nil {
			continue
		}

		switch eventType.Type {
		case "content_block_delta":
			var ev struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			if ev.Delta.Text != "" {
				ch <- StreamChunk{Delta: ev.Delta.Text}
			}

		case "message_delta":
			var ev struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			sc := StreamChunk{}
			if ev.Delta.StopReason != "" {
				sc.StopReason = ev.Delta.StopReason
			}
			if ev.Usage.InputTokens > 0 || ev.Usage.OutputTokens > 0 {
				sc.Usage = &Usage{
					PromptTokens:     ev.Usage.InputTokens,
					CompletionTokens: ev.Usage.OutputTokens,
				}
			}
			ch <- sc
		}
	}
}
