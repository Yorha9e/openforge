package llm

import (
	"fmt"
	"sync"
)

type ModelEntry struct {
	Alias        string
	Provider     string
	ModelID      string
	BaseURL      string
	KeyRef       string
	FeatureFlags FeatureFlags
	Fallback     []string
}

type FeatureFlags struct {
	SupportsToolUse   bool
	SupportsStreaming bool
	SupportsThinking  bool
	MaxTokens         int
}

type Registry struct {
	mu      sync.RWMutex
	entries map[string]*ModelEntry
}

func NewRegistry() *Registry {
	r := &Registry{entries: make(map[string]*ModelEntry)}
	r.seedDefaults()
	return r
}

func (r *Registry) seedDefaults() {
	r.Register(&ModelEntry{
		Alias: "sonnet", Provider: "anthropic", ModelID: "claude-sonnet-4-6-20250514",
		BaseURL: "https://api.anthropic.com", KeyRef: "ANTHROPIC_AUTH_TOKEN",
		FeatureFlags: FeatureFlags{SupportsToolUse: true, SupportsStreaming: true, MaxTokens: 200000},
		Fallback:     []string{"deepseek", "haiku"},
	})
	r.Register(&ModelEntry{
		Alias: "haiku", Provider: "anthropic", ModelID: "claude-haiku-4-5-20251001",
		BaseURL: "https://api.anthropic.com", KeyRef: "ANTHROPIC_AUTH_TOKEN",
		FeatureFlags: FeatureFlags{SupportsToolUse: true, SupportsStreaming: true, MaxTokens: 200000},
	})
	r.Register(&ModelEntry{
		Alias: "deepseek", Provider: "deepseek", ModelID: "deepseek-v4-pro[1m]",
		BaseURL: "https://api.deepseek.com/anthropic", KeyRef: "ANTHROPIC_AUTH_TOKEN",
		FeatureFlags: FeatureFlags{SupportsToolUse: true, SupportsStreaming: true, MaxTokens: 128000},
	})
	r.Register(&ModelEntry{
		Alias: "ollama", Provider: "ollama", ModelID: "qwen3",
		BaseURL: "http://localhost:11434", KeyRef: "",
		FeatureFlags: FeatureFlags{SupportsToolUse: true, SupportsStreaming: true, MaxTokens: 32000},
	})
}

func (r *Registry) Lookup(alias string) (*ModelEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[alias]
	if !ok {
		return nil, fmt.Errorf("model %q not found", alias)
	}
	return e, nil
}

func (r *Registry) Register(e *ModelEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.Alias] = e
}

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
			KeyRef:   "ANTHROPIC_AUTH_TOKEN",
			Fallback: m.Fallback,
			FeatureFlags: FeatureFlags{
				SupportsToolUse:   true,
				SupportsStreaming: true,
				MaxTokens:         200000,
			},
		}
	}
}

// ModelDef defines a model entry for loading from config (avoids circular import).
type ModelDef struct {
	Alias    string
	Provider string
	ModelID  string
	BaseURL  string
	Fallback []string
}

func (r *Registry) List() []*ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ModelEntry, 0, len(r.entries))
	for _, e := range r.entries {
		result = append(result, e)
	}
	return result
}
