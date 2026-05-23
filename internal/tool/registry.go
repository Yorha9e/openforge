package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"openforge/internal/agent/port"
)

// Tool is a simple tool with a Run method.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Run(ctx context.Context, input []byte) ([]byte, error)
}

// Registry implements port.ToolSearcher + port.ToolRunner with keyword-based search.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool         // name → Tool (executable)
	infos map[string]port.ToolInfo // name → ToolInfo (search-only)
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
		infos: make(map[string]port.ToolInfo),
	}
}

func (r *Registry) Register(ctx context.Context, info port.ToolInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.infos[info.Name] = info
	return nil
}

func (r *Registry) RegisterTool(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
	r.infos[t.Name()] = port.ToolInfo{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: t.InputSchema(),
	}
}

// Search performs keyword-based tool matching.
func (r *Registry) Search(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keywords := strings.Fields(strings.ToLower(query))
	var results []port.ToolMatch

	for name, info := range r.infos {
		text := strings.ToLower(name + " " + info.Description)
		score := r.matchScore(keywords, text)
		if score > 0 {
			results = append(results, port.ToolMatch{
				Name:        name,
				Description: info.Description,
				Score:       score,
			})
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// SearchTools is the backward-compatible alias.
func (r *Registry) SearchTools(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	return r.Search(ctx, query, topK)
}

func (r *Registry) matchScore(keywords []string, text string) float64 {
	hits := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(keywords))
}

func (r *Registry) List(ctx context.Context) ([]port.ToolInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []port.ToolInfo
	for _, info := range r.infos {
		result = append(result, info)
	}
	return result, nil
}

func (r *Registry) Run(ctx context.Context, call port.ToolCall) (port.ToolResult, error) {
	r.mu.RLock()
	t, ok := r.tools[call.ToolName]
	r.mu.RUnlock()
	if !ok {
		return port.ToolResult{}, fmt.Errorf("tool %q not found", call.ToolName)
	}

	output, err := t.Run(ctx, call.Input)
	if err != nil {
		return port.ToolResult{Error: err.Error()}, nil
	}
	return port.ToolResult{Output: output}, nil
}

// EchoTool is a simple test tool that returns the input as output.
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo" }
func (t *EchoTool) Description() string { return "Echo back the input as output (for testing)" }
func (t *EchoTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"message": map[string]string{"type": "string"}},
	}
}
func (t *EchoTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(input, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
