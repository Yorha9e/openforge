package llm

import (
	"context"
	"fmt"

	"openforge/internal/agent/port"
	"openforge/internal/shared/kernel"
)

type Router struct {
	registry  *Registry
	providers map[string]Provider
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

func (r *Router) getProvider(entry *ModelEntry) (Provider, error) {
	p, ok := r.providers[entry.Provider]
	if !ok {
		return nil, fmt.Errorf("no provider registered for %q", entry.Provider)
	}
	return p, nil
}

func (r *Router) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	entry, err := r.registry.Lookup(req.Config.Model)
	if err != nil {
		return nil, err
	}

	// Validate provider exists (chatWithFallback re-fetches it)
	if _, err := r.getProvider(entry); err != nil {
		return nil, err
	}

	llmReq := ChatRequest{
		Tools:        convertTools(req.Tools),
		Model:        entry.ModelID,
		Messages:     convertMessages(req.Messages),
		SystemPrompt: req.SystemPrompt,
		MaxTokens:    req.Config.MaxTokens,
	}

	resp, err := r.chatWithFallback(ctx, entry, llmReq)
	if err != nil {
		return nil, err
	}

	return &port.ChatResponse{
		Content:    resp.Content,
		StopReason: resp.StopReason,
		Usage: &port.Usage{
			InputTokens:  int64(resp.Usage.PromptTokens),
			OutputTokens: int64(resp.Usage.CompletionTokens),
		},
	}, nil
}

func (r *Router) chatWithFallback(ctx context.Context, entry *ModelEntry, req ChatRequest) (ChatResponse, error) {
	provider, err := r.getProvider(entry)
	if err != nil {
		return ChatResponse{}, err
	}

	resp, err := provider.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}

	for _, fbAlias := range entry.Fallback {
		fbEntry, lookupErr := r.registry.Lookup(fbAlias)
		if lookupErr != nil {
			continue
		}
		fbReq := req
		fbReq.Model = fbEntry.ModelID
		fbProvider, fbErr := r.getProvider(fbEntry)
		if fbErr != nil {
			continue
		}
		fbResp, fbErr := fbProvider.Chat(ctx, fbReq)
		if fbErr == nil {
			return fbResp, nil
		}
	}
	return ChatResponse{}, fmt.Errorf("all providers exhausted: %w", err)
}

func (r *Router) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan port.StreamChunk, error) {
	entry, err := r.registry.Lookup(req.Config.Model)
	if err != nil {
		return nil, err
	}

	provider, err := r.getProvider(entry)
	if err != nil {
		return nil, err
	}

	llmReq := ChatRequest{
		Tools:        convertTools(req.Tools),
		Model:        entry.ModelID,
		Messages:     convertMessages(req.Messages),
		SystemPrompt: req.SystemPrompt,
		MaxTokens:    req.Config.MaxTokens,
	}

	streamCh, err := provider.ChatStream(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	// Convert <-chan llm.StreamChunk to <-chan port.StreamChunk
	ch := make(chan port.StreamChunk, 64)
	go func() {
		defer close(ch)
		for chunk := range streamCh {
			var usage *port.Usage
			if chunk.Usage != nil {
				usage = &port.Usage{
					InputTokens:  int64(chunk.Usage.PromptTokens),
					OutputTokens: int64(chunk.Usage.CompletionTokens),
				}
			}
			ch <- port.StreamChunk{
				Delta:        chunk.Delta,
				FinishReason: chunk.StopReason,
				Usage:        usage,
			}
		}
	}()
	return ch, nil
}

func (r *Router) Close() error { return nil }

// ModelInfo is a lightweight model descriptor for the API.
type ModelInfo struct {
	Alias    string `json:"alias"`
	Provider string `json:"provider"`
	ModelID  string `json:"model_id"`
}

// ListModels returns all registered model entries for the model selector API.
func (r *Router) ListModels() []ModelInfo {
	entries := r.registry.List()
	result := make([]ModelInfo, 0, len(entries))
	for _, e := range entries {
		result = append(result, ModelInfo{
			Alias:    e.Alias,
			Provider: e.Provider,
			ModelID:  e.ModelID,
		})
	}
	return result
}

func convertMessages(msgs []port.Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = Message{Role: m.Role, Content: m.Content}
	}
	return out
}

func convertTools(tools []port.ToolDef) []ToolDef {
	out := make([]ToolDef, len(tools))
	for i, t := range tools {
		out[i] = ToolDef{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}
	return out
}
