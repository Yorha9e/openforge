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

	"openforge/internal/agent/port"
	"openforge/internal/shared/kernel"
)

type Router struct {
	registry *Registry
	secrets  kernel.SecretStore
	client   *http.Client
}

func NewRouter(registry *Registry, secrets kernel.SecretStore) *Router {
	return &Router{registry: registry, secrets: secrets, client: &http.Client{}}
}

func (r *Router) Close() error { return nil }

func (r *Router) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	entry, err := r.registry.Lookup(req.Config.Model)
	if err != nil {
		return nil, err
	}

	apiKey, err := r.secrets.Get(ctx, entry.KeyRef)
	if err != nil {
		return nil, fmt.Errorf("api key: %w", err)
	}

	requestBody := r.buildAnthropicRequestBody(req, entry)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", entry.BaseURL+"/v1/messages", requestBody)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", string(apiKey))
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm %d: %s", resp.StatusCode, string(b))
	}
	return r.parseAnthropicResponse(resp.Body)
}

func (r *Router) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan string, error) {
	entry, err := r.registry.Lookup(req.Config.Model)
	if err != nil {
		return nil, err
	}

	apiKey, err := r.secrets.Get(ctx, entry.KeyRef)
	if err != nil {
		return nil, fmt.Errorf("api key: %w", err)
	}

	bodyMap := r.buildAnthropicRequestBodyMap(req, entry)
	bodyMap["stream"] = true
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", entry.BaseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", string(apiKey))
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm stream: %w", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("llm stream %d: %s", resp.StatusCode, string(b))
	}

	ch := make(chan string, 64)
	go r.streamSSE(resp.Body, ch)
	return ch, nil
}

func (r *Router) buildAnthropicRequestBody(req port.ChatRequest, entry *ModelEntry) *bytes.Buffer {
	bodyMap := r.buildAnthropicRequestBodyMap(req, entry)
	b, _ := json.Marshal(bodyMap)
	return bytes.NewBuffer(b)
}

func (r *Router) buildAnthropicRequestBodyMap(req port.ChatRequest, entry *ModelEntry) map[string]interface{} {
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]interface{}{"role": m.Role, "content": m.Content}
	}
	return map[string]interface{}{
		"model":      entry.ModelID,
		"max_tokens": req.Config.MaxTokens,
		"messages":   messages,
	}
}

func (r *Router) parseAnthropicResponse(body io.Reader) (*port.ChatResponse, error) {
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, err
	}
	var text string
	for _, c := range result.Content {
		text += c.Text
	}
	return &port.ChatResponse{Content: text}, nil
}

func (r *Router) streamSSE(body io.ReadCloser, ch chan<- string) {
	defer body.Close()
	defer close(ch)
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(data), &event) == nil {
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				ch <- event.Delta.Text
			}
		}
	}
}
