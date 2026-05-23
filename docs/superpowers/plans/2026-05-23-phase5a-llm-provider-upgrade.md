# Phase 5a — LLM Router Provider Abstraction + Model Config YAML

> **状态: ✅ 已完成**

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** LLM Router 从"硬编码 Anthropic 格式"升级为"可扩展提供者抽象层"，支持 OpenAI/Gemini 协议翻译，模型注册表从 YAML 加载，模型回退链。

**Architecture:** 参考 DESIGN.md §6 LLM Router 设计。新增 `Provider` 接口（`Chat`/`ChatStream`），Anthropic/OpenAI/Gemini 各自实现。`Translator` 层负责 Anthropic `/v1/messages` 格式 ←→ 各提供者原生格式的转换。模型注册表从 `minimal.yaml` 的 `models: []` 列表加载，支持 `fallback[]` 链。Go 侧路由器和 Node.js IO 层同步升级。

**Tech Stack:** Go 1.25 + `net/http`, TypeScript + `@anthropic-ai/sdk` + `openai` npm, ConnectRPC

**关键约束:**
- Go 侧直连 Anthropic/DeepSeek（兼容 /v1/messages 格式，零翻译）
- Go 侧 OpenAI/Gemini 走 Translator 翻译层
- Node.js IO 层同步新增 OpenAI/Gemini provider
- 模型回退链：primary → fallback[0] → fallback[1]
- YAML 模型声明覆盖硬编码种子

---
## File Map

```
openforge/
├── internal/llm/
│   ├── provider.go              # NEW: Provider 接口 (Chat + ChatStream)
│   ├── anthropic_provider.go    # NEW: Anthropic 直通 (无翻译)
│   ├── deepseek_provider.go     # NEW: DeepSeek 直通 (兼容 /v1/messages)
│   ├── openai_provider.go       # NEW: OpenAI → 经 Translator 翻译
│   ├── translator.go            # NEW: Anthropic→OpenAI/Google 请求翻译层
│   ├── translator_test.go       # NEW: translator 单元测试
│   ├── registry.go              # MODIFY: 从 YAML 加载替代硬编码种子
│   ├── router.go                # MODIFY: 注入 Provider map + 回退链
│   └── registry_test.go         # MODIFY: 测试 YAML 加载
├── config/profiles/
│   └── minimal.yaml             # MODIFY: llm 块扩展 models: [] + defaults
├── internal/shared/profile/
│   └── loader.go                # MODIFY: LLMConfig 增加 Models []ModelEntry
└── nodejs-io/src/
    ├── llm/
    │   ├── providers/
    │   │   ├── openai.ts         # NEW: OpenAI SDK provider
    │   │   └── deepseek.ts       # NEW: DeepSeek provider
    │   └── translator.ts         # NEW: TS 侧翻译层 (备用)
    └── server.ts                 # MODIFY: 注册新 provider 路由
```

---

### Task 1: Provider 接口 + Anthropic/DeepSeek 直通实现

> 定义 Go 侧 `Provider` 接口，把现有 Router 里的 Anthropic HTTP 逻辑抽到 `AnthropicProvider`，新增直通的 `DeepSeekProvider`

**Files:**
- Create: `internal/llm/provider.go`
- Create: `internal/llm/anthropic_provider.go`
- Create: `internal/llm/deepseek_provider.go`
- Modify: `internal/llm/router.go` — 注入 Provider map

- [ ] **Step 1: 写 Provider 接口**

Create `internal/llm/provider.go`:

```go
package llm

import "context"

// ChatRequest is a normalized request for all providers.
type ChatRequest struct {
	Model    string
	Messages []Message
	MaxTokens int
	SystemPrompt string
}

// Message is a normalized chat message.
type Message struct {
	Role    string // "user" | "assistant" | "system"
	Content string
}

// ChatResponse is a normalized response.
type ChatResponse struct {
	Content   string
	StopReason string
	Usage     Usage
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
}

// Provider abstracts an LLM provider backend.
type Provider interface {
	// Chat sends a request and returns the full response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream sends a request and streams response chunks.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}
```

- [ ] **Step 2: 跑测试 — FAIL**

```bash
go build ./internal/llm/...
```
Expected: no errors (provider.go only defines interfaces)

- [ ] **Step 3: 实现 AnthropicProvider**

Create `internal/llm/anthropic_provider.go`:

```go
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
		Content []struct{ Text string }
		StopReason string `json:"stop_reason"`
		Usage struct {
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
		Content:   text,
		StopReason: result.StopReason,
		Usage:     Usage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens},
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
		var ev struct {
			Type  string `json:"type"`
			Delta struct{ Text string `json:"text"` }
			Message struct{ StopReason string `json:"stop_reason"` }
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "content_block_delta":
			ch <- StreamChunk{Delta: ev.Delta.Text}
		case "message_stop":
			ch <- StreamChunk{StopReason: ev.Message.StopReason}
		}
	}
}
```

- [ ] **Step 4: 实现 DeepSeekProvider**

Create `internal/llm/deepseek_provider.go`:

```go
package llm

// DeepSeekProvider reuses AnthropicProvider since DeepSeek API is
// Anthropic Messages API compatible (base URL differs).
type DeepSeekProvider struct {
	*AnthropicProvider
}

func NewDeepSeekProvider(baseURL, apiKey string) *DeepSeekProvider {
	return &DeepSeekProvider{AnthropicProvider: NewAnthropicProvider(baseURL, apiKey)}
}
```

DeepSeek 的 API 完全兼容 Anthropic `/v1/messages` 格式，只需更换 BaseURL 和 APIKey。

- [ ] **Step 5: 修改 Router 注入 Provider map**

In `internal/llm/router.go`, replace the hardcoded HTTP logic with provider delegation:

```go
type Router struct {
	registry  *Registry
	providers map[string]Provider // key: provider name ("anthropic","deepseek","openai")
	secrets   kernel.SecretStore
}

func NewRouter(reg *Registry, secrets kernel.SecretStore) *Router {
	return &Router{
		registry:  reg,
		providers: make(map[string]Provider),
		secrets:   secrets,
	}
}

// RegisterProvider adds or replaces a provider backend.
func (r *Router) RegisterProvider(name string, p Provider) {
	r.providers[name] = p
}

func (r *Router) getProvider(entry ModelEntry) (Provider, error) {
	p, ok := r.providers[entry.Provider]
	if !ok {
		return nil, fmt.Errorf("no provider registered for %q", entry.Provider)
	}
	return p, nil
}
```

- [ ] **Step 6: 更新 Bootstrap 注入 Anthropic/DeepSeek providers**

In `bootstrap.go` `Bootstrap()`:

```go
// Register LLM providers
antAPIKey, _ := of.Secrets.Get(context.Background(), "ANTHROPIC_AUTH_TOKEN")
dsAPIKey, _ := of.Secrets.Get(context.Background(), "DEEPSEEK_AUTH_TOKEN")

of.LLMRouter.RegisterProvider("anthropic", llm.NewAnthropicProvider(
    "https://api.anthropic.com", string(antAPIKey)))
of.LLMRouter.RegisterProvider("deepseek", llm.NewDeepSeekProvider(
    "https://api.deepseek.com/anthropic", string(dsAPIKey)))
```

- [ ] **Step 7: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/llm/... -count=1
```

```bash
git add internal/llm/provider.go internal/llm/anthropic_provider.go internal/llm/deepseek_provider.go internal/llm/router.go internal/shared/profile/bootstrap.go
git commit -m "feat(llm): add Provider interface with Anthropic/DeepSeek implementations"
```

---

### Task 2: Translator — Anthropic ↔ OpenAI/Gemini 翻译层

> 实现请求/响应翻译，让 OpenAI 和 Gemini 提供者能接受 Anthropic Messages API 格式
> 
> Translation table per DESIGN.md §6.4:
> - Anthropic `system` → OpenAI: first message with `role: "system"`
> - Anthropic `messages[].role` → OpenAI: `assistant` ↔ `assistant`, `user` ↔ `user`
> - Anthropic `stop_reason: "end_turn"` → OpenAI: `finish_reason: "stop"`
> - Anthropic `max_tokens` → OpenAI: `max_tokens`
> - Gemini: Anthropic→Google `GenerateContentRequest` 格式转换
> - Streaming: SSE → SSE, 不同 event type 映射

**Files:**
- Create: `internal/llm/translator.go`
- Create: `internal/llm/translator_test.go`
- Create: `internal/llm/openai_provider.go`

- [ ] **Step 1: 写 Translator 测试**

Create `internal/llm/translator_test.go`:

```go
package llm

import (
	"testing"
)

func TestTranslateToOpenAI(t *testing.T) {
	req := ChatRequest{
		Model:        "gpt-4o",
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		MaxTokens: 4096,
	}

	tr := NewTranslator()
	openaiBody := tr.ToOpenAI(req)

	// Check system message inserted as first message
	messages := openaiBody["messages"].([]map[string]string)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (system + 2), got %d", len(messages))
	}
	if messages[0]["role"] != "system" {
		t.Errorf("first message role = %q, want system", messages[0]["role"])
	}
	if messages[0]["content"] != "You are helpful." {
		t.Errorf("system content = %q", messages[0]["content"])
	}
}

func TestTranslateOpenAIResponse(t *testing.T) {
	openaiResp := openaiChatResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{Message: struct{ Content string }{Content: "Hello!"}, FinishReason: "stop"}},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		}{PromptTokens: 10, CompletionTokens: 5},
	}

	tr := NewTranslator()
	resp := tr.FromOpenAI(openaiResp)

	if resp.Content != "Hello!" {
		t.Errorf("content = %q, want Hello!", resp.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want end_turn", resp.StopReason)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens = %d, want 10", resp.Usage.PromptTokens)
	}
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/llm/ -v -run TestTranslate -count=1
```
Expected: FAIL — NewTranslator not defined

- [ ] **Step 3: 实现 Translator**

Create `internal/llm/translator.go`:

```go
package llm

import (
	"encoding/json"
)

// openaiChatResponse is the OpenAI chat completion response shape.
type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Translator converts between Anthropic Messages format and
// other provider formats (OpenAI, Google Gemini).
type Translator struct{}

func NewTranslator() *Translator { return &Translator{} }

// ToOpenAI converts an Anthropic-format ChatRequest to an OpenAI
// chat completion request body.
func (t *Translator) ToOpenAI(req ChatRequest) map[string]interface{} {
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role": "system", "content": req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, map[string]string{
			"role": role, "content": m.Content,
		})
	}
	return map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"max_tokens": req.MaxTokens,
	}
}

// FromOpenAI converts an OpenAI chat completion response to a ChatResponse.
func (t *Translator) FromOpenAI(resp openaiChatResponse) ChatResponse {
	content := ""
	stopReason := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		switch resp.Choices[0].FinishReason {
		case "stop":
			stopReason = "end_turn"
		case "length":
			stopReason = "max_tokens"
		default:
			stopReason = resp.Choices[0].FinishReason
		}
	}
	return ChatResponse{
		Content:    content,
		StopReason: stopReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		},
	}
}

// ToGemini converts to Google Gemini GenerateContentRequest format.
func (t *Translator) ToGemini(req ChatRequest) map[string]interface{} {
	contents := make([]map[string]interface{}, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	payload := map[string]interface{}{
		"contents": contents,
	}
	if req.SystemPrompt != "" {
		payload["system_instruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": req.SystemPrompt}},
		}  
	}
	return payload
}

// FromGemini converts a Gemini response to ChatResponse.
func (t *Translator) FromGemini(body []byte) (ChatResponse, error) {
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct{ Text string }
				Role  string
			}
			FinishReason string `json:"finishReason"`
		}
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ChatResponse{}, err
	}
	text := ""
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		text = resp.Candidates[0].Content.Parts[0].Text
	}
	return ChatResponse{
		Content:    text,
		StopReason: resp.Candidates[0].FinishReason,
		Usage: Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}
```

- [ ] **Step 4: 运行测试 — PASS**

```bash
go test ./internal/llm/ -v -run TestTranslate -count=1
```
Expected: PASS

- [ ] **Step 5: 实现 OpenAIProvider (用 Translator)**

Create `internal/llm/openai_provider.go`:

```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	var openaiResp openaiChatResponse
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
	decoder := json.NewDecoder(r)
	for {
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := decoder.Decode(&ev); err != nil {
			break
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
```

- [ ] **Step 6: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/llm/... -count=1
```

```bash
git add internal/llm/translator.go internal/llm/translator_test.go internal/llm/openai_provider.go
git commit -m "feat(llm): add Anthropic→OpenAI/Gemini translator and OpenAI provider"
```

---

### Task 3: Model Registry YAML 加载 + 回退链

> 把硬编码在 `registry.go` 里的 4 个种子模型移到 `minimal.yaml`，支持 fallback 链

**Files:**
- Modify: `internal/shared/profile/loader.go` — LLMConfig 扩展
- Modify: `config/profiles/minimal.yaml` — llm 块增加 models
- Modify: `internal/llm/registry.go` — 从 Config.Models 加载
- Modify: `internal/llm/router.go` — 回退链逻辑
- Modify: `internal/llm/registry_test.go` — 适配新加载方式

- [ ] **Step 1: 扩展 LLMConfig**

In `internal/shared/profile/loader.go`, replace the `LLMConfig` struct:

```go
type LLMConfig struct {
	DefaultProvider string      `yaml:"default_provider"`
	DefaultModel    string      `yaml:"default_model"`
	Models          []ModelDef  `yaml:"models"`
}

type ModelDef struct {
	Alias    string   `yaml:"alias"`
	Provider string   `yaml:"provider"`
	ModelID  string   `yaml:"model_id"`
	BaseURL  string   `yaml:"base_url"`
	Fallback []string `yaml:"fallback"`
}
```

- [ ] **Step 2: 更新 minimal.yaml**

Replace the `llm:` block:

```yaml
llm:
  default_provider: deepseek
  default_model: deepseek
  models:
    - alias: sonnet
      provider: anthropic
      model_id: claude-sonnet-4-6-20250514
      base_url: https://api.anthropic.com
      fallback: [haiku]
    - alias: haiku
      provider: anthropic
      model_id: claude-haiku-4-5-20251001
      base_url: https://api.anthropic.com
    - alias: deepseek
      provider: deepseek
      model_id: deepseek-v4-pro[1m]
      base_url: https://api.deepseek.com/anthropic
      fallback: [sonnet]
    - alias: ollama
      provider: ollama
      model_id: qwen3
      base_url: http://localhost:11434
```

- [ ] **Step 3: 修改 Registry 从 Config 加载**

In `internal/llm/registry.go`, add a `LoadFromConfig` method:

```go
// LoadFromConfig populates the registry from profile config.
// This replaces the hardcoded seed entries.
func (r *Registry) LoadFromConfig(models []ModelDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range models {
		r.entries[m.Alias] = &ModelEntry{
			Alias:    m.Alias,
			Provider: m.Provider,
			ModelID:  m.ModelID,
			BaseURL:  m.BaseURL,
			Fallback: m.Fallback,
		}
	}
}
```

Remove the `seed()` calls from `NewRegistry()`. The registry starts empty and loads from config in `Bootstrap()`.

- [ ] **Step 4: Router 回退链逻辑**

In `internal/llm/router.go`, add retry with fallback to `Chat()` and `ChatStream()`:

```go
func (r *Router) Chat(ctx context.Context, modelKey string, req ChatRequest) (ChatResponse, error) {
	entry, err := r.registry.Lookup(modelKey)
	if err != nil {
		return ChatResponse{}, err
	}
	return r.chatWithFallback(ctx, entry, req, 0)
}

func (r *Router) chatWithFallback(ctx context.Context, entry *ModelEntry, req ChatRequest, depth int) (ChatResponse, error) {
	provider, err := r.getProvider(*entry)
	if err != nil {
		return ChatResponse{}, err
	}

	resp, err := provider.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}

	// Try fallback chain
	for i, fbAlias := range entry.Fallback {
		if i+depth >= 3 {
			break // max 3 fallback hops
		}
		fbEntry, lookupErr := r.registry.Lookup(fbAlias)
		if lookupErr != nil {
			continue
		}
		fbReq := req
		fbReq.Model = fbEntry.ModelID
		fbResp, fbErr := r.chatWithFallback(ctx, fbEntry, fbReq, depth+i+1)
		if fbErr == nil {
			return fbResp, nil
		}
	}
	return ChatResponse{}, fmt.Errorf("all providers exhausted: %w", err)
}
```

- [ ] **Step 5: Bootstrap 加载模型列表 + 注册 OpenAI provider**

```go
of.LLMRouter = llm.NewRouter(llmRegistry, of.Secrets)
llmRegistry.LoadFromConfig(cfg.LLM.Models)
// ... provider registration ...
```

- [ ] **Step 6: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
go test ./internal/llm/... -count=1
```

```bash
git add internal/llm/registry.go internal/llm/router.go internal/llm/registry_test.go internal/shared/profile/loader.go config/profiles/minimal.yaml
git commit -m "feat(llm): load model registry from YAML config with fallback chain support"
```

---

### Task 4: Node.js IO 层 OpenAI + DeepSeek Provider

> TS 侧实现 OpenAI 和 DeepSeek provider，支持非流式 + 流式调用

**Files:**
- Create: `nodejs-io/src/llm/providers/openai.ts`
- Create: `nodejs-io/src/llm/providers/deepseek.ts`
- Modify: `nodejs-io/src/server.ts` — 注册 OpenAI provider 路由

- [ ] **Step 1: 实现 OpenAI provider**

Create `nodejs-io/src/llm/providers/openai.ts`:

```typescript
import OpenAI from 'openai';
import type { ChatRequest, ChatResponse, StreamChunk, LLMProvider } from '../../kernel/interfaces.js';

export class OpenAIProvider implements LLMProvider {
  private client: OpenAI;

  constructor(baseURL: string, apiKey: string) {
    this.client = new OpenAI({ baseURL, apiKey });
  }

  async chat(req: ChatRequest): Promise<ChatResponse> {
    const resp = await this.client.chat.completions.create({
      model: req.model_id,
      messages: this.buildMessages(req),
      max_tokens: req.max_tokens,
    });
    return {
      content: resp.choices[0]?.message?.content ?? '',
      stop_reason: resp.choices[0]?.finish_reason ?? '',
      usage: { prompt_tokens: resp.usage?.prompt_tokens ?? 0, completion_tokens: resp.usage?.completion_tokens ?? 0 },
    };
  }

  async *chatStream(req: ChatRequest): AsyncGenerator<StreamChunk> {
    const stream = await this.client.chat.completions.create({
      model: req.model_id,
      messages: this.buildMessages(req),
      max_tokens: req.max_tokens,
      stream: true,
    });
    for await (const chunk of stream) {
      const delta = chunk.choices[0]?.delta?.content;
      if (delta) yield { delta };
      if (chunk.choices[0]?.finish_reason) {
        yield { stop_reason: chunk.choices[0].finish_reason };
      }
    }
  }

  private buildMessages(req: ChatRequest): OpenAI.Chat.Completions.ChatCompletionMessageParam[] {
    const msgs: OpenAI.Chat.Completions.ChatCompletionMessageParam[] = [];
    if (req.system_prompt) {
      msgs.push({ role: 'system', content: req.system_prompt });
    }
    for (const m of req.messages) {
      msgs.push({ role: m.role as 'user' | 'assistant', content: m.content });
    }
    return msgs;
  }
}
```

- [ ] **Step 2: 实现 DeepSeek provider**

Create `nodejs-io/src/llm/providers/deepseek.ts`:

```typescript
import { AnthropicProvider } from './anthropic.js';

// DeepSeek API is Anthropic Messages API compatible.
// Reuse AnthropicProvider with a different base URL.
export class DeepSeekProvider extends AnthropicProvider {
  constructor(baseURL: string, apiKey: string) {
    super(baseURL, apiKey);
  }
}
```

- [ ] **Step 3: 在 server.ts 注册**

```typescript
import { OpenAIProvider } from './llm/providers/openai.js';
import { DeepSeekProvider } from './llm/providers/deepseek.js';

const openaiProvider = new OpenAIProvider(
  process.env.OPENAI_BASE_URL ?? 'https://api.openai.com',
  process.env.OPENAI_API_KEY ?? ''
);
const deepseekProvider = new DeepSeekProvider(
  'https://api.deepseek.com/anthropic',
  process.env.DEEPSEEK_API_KEY ?? ''
);
```

- [ ] **Step 4: 编译验证 + Commit**

```bash
cd nodejs-io && npx tsc --noEmit && npm run build
```

```bash
git add nodejs-io/
git commit -m "feat(nodejs): add OpenAI and DeepSeek LLM providers"
```

---

### Task 5: Frontend — Model Selector UI

> 在聊天面板添加模型切换下拉菜单，调用 `/api/models` 列出可用模型

**Files:**
- Create: `frontend/src/features/chat/ModelSelector.tsx`
- Modify: `frontend/src/features/chat/ChatPanel.tsx` — 集成 ModelSelector
- Modify: `frontend/src/shared/api.ts` — 加 listModels API

- [ ] **Step 1: 加 listModels API**

In `frontend/src/shared/api.ts`:

```typescript
  listModels: () => request<any[]>('/models'),
```

- [ ] **Step 2: 创建 ModelSelector**

Create `frontend/src/features/chat/ModelSelector.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';

interface ModelSelectorProps {
  current: string;
  onSelect: (model: string) => void;
}

export function ModelSelector({ current, onSelect }: ModelSelectorProps) {
  const [models, setModels] = useState<any[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => { api.listModels().then(setModels).catch(() => {}); }, []);

  return (
    <div style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen(!open)}
        style={{
          background: tokens.surface, border: `1px solid ${tokens.border}`,
          borderRadius: 6, padding: '6px 12px', color: tokens.text,
          fontFamily: tokens.fontBody, fontSize: 13, cursor: 'pointer',
          transition: tokens.transition,
        }}
      >{current} ▾</button>
      {open && (
        <div style={{
          position: 'absolute', top: '100%', right: 0, marginTop: 4,
          background: tokens.surface, border: `1px solid ${tokens.border}`,
          borderRadius: 6, minWidth: 180, zIndex: 50, overflow: 'hidden',
        }}>
          {models.map(m => (
            <button key={m.alias}
              onClick={() => { onSelect(m.alias); setOpen(false); }}
              style={{
                display: 'block', width: '100%', padding: '8px 12px',
                background: m.alias === current ? tokens.bg : 'transparent',
                border: 'none', color: m.alias === current ? tokens.cta : tokens.text,
                fontFamily: tokens.fontBody, fontSize: 13, cursor: 'pointer',
                textAlign: 'left', transition: tokens.transition,
              }}
            >{m.alias} <span style={{ color: tokens.muted, fontSize: 11 }}>{m.provider}</span></button>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: 集成到 ChatPanel**

Import and add to ChatPanel header:

```tsx
import { ModelSelector } from './ModelSelector';
// In header: <ModelSelector current={model} onSelect={setModel} />
```

- [ ] **Step 4: 后端加 /api/models 路由**

In `routes.go`:

```go
mux.HandleFunc("GET /api/models", authMw(handleListModels(of)))

func handleListModels(of *profile.OpenForge) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        models := of.LLMRouter.ListModels()
        writeJSON(w, 200, models)
    }
}
```

In `router.go`, add:
```go
func (r *Router) ListModels() []ModelInfo {
    entries := r.registry.List()
    result := make([]ModelInfo, 0, len(entries))
    for _, e := range entries {
        result = append(result, ModelInfo{Alias: e.Alias, Provider: e.Provider, ModelID: e.ModelID})
    }
    return result
}
```

- [ ] **Step 5: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/
cd frontend && npx tsc --noEmit
```

```bash
git add internal/llm/router.go internal/server/routes.go frontend/src/features/chat/ModelSelector.tsx frontend/src/features/chat/ChatPanel.tsx frontend/src/shared/api.ts
git commit -m "feat(frontend): add model selector dropdown in chat panel"
```

---

### Task 6: E2E 验证

- [ ] **Step 1: Go 测试 + 编译**

```bash
go build ./cmd/server/
go test ./internal/llm/... ./internal/server/... -count=1
```

- [ ] **Step 2: 前端编译 + 构建**

```bash
cd frontend && npx tsc --noEmit && npm run build
```

- [ ] **Step 3: Node.js IO 层编译**

```bash
cd nodejs-io && npx tsc --noEmit && npm run build
```

- [ ] **Step 4: 全栈启动验证**

```bash
# Terminal 1: Go
$env:ANTHROPIC_AUTH_TOKEN = "sk-xxx"
go run ./cmd/server/ --addr :8030

# Terminal 2: Node.js IO (optional, only if gRPC features tested)
cd nodejs-io && npm start

# Terminal 3: Frontend
cd frontend && npm run dev
```

验证：模型选择器在下拉列表中显示所有 4 个模型，启动日志显示从 YAML 加载注册表

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore(phase5a): final verification — all tests pass, frontend builds"
```

---

## Phase 5a Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | `go build ./cmd/server/` | automated |
| 2 | `go test ./internal/llm/...` PASS | automated |
| 3 | 模型注册表从 minimal.yaml 加载 | 日志 |
| 4 | Translator Anthropic→OpenAI 请求格式正确 | automated (test) |
| 5 | OpenAI provider 响应翻译回 ChatResponse 正确 | automated (test) |
| 6 | 回退链: 主模型失败 → fallback[0] → fallback[1] | manual |
| 7 | `npm run build` 前端 + Node.js 零错误 | automated |
| 8 | 模型选择器 UI 列出 4 个模型 | visual |
| 9 | Provider 接口统一: Anthropic/DeepSeek/OpenAI 三个实现 | code |
