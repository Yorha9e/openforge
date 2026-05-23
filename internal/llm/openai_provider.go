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

// OpenAIProvider implements Provider via OpenAI Chat Completions API.
type OpenAIProvider struct {
	BaseURL    string
	APIKey     string
	client     *http.Client
	translator *Translator
}

func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		client:     &http.Client{},
		translator: NewTranslator(),
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := p.translator.ToOpenAI(req)
	b, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("openai: %d %s", resp.StatusCode, string(b))
	}

	var openaiResp OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return ChatResponse{}, err
	}
	return p.translator.FromOpenAI(openaiResp), nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	body := p.translator.ToOpenAI(req)
	body["stream"] = true
	b, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai stream: %d %s", resp.StatusCode, string(b))
	}

	ch := make(chan StreamChunk, 64)
	go p.readOpenAISSE(resp.Body, ch)
	return ch, nil
}

func (p *OpenAIProvider) readOpenAISSE(r io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		if len(ev.Choices) > 0 {
			if ev.Choices[0].Delta.Content != "" {
				ch <- StreamChunk{Delta: ev.Choices[0].Delta.Content}
			}
			if ev.Choices[0].FinishReason != "" {
				ch <- StreamChunk{StopReason: ev.Choices[0].FinishReason}
			}
		}
	}
}
