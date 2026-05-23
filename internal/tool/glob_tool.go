package tool

import (
	"context"
	"encoding/json"
	"path/filepath"
)

type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Find files matching a glob pattern" }
func (t *GlobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Glob pattern (e.g., **/*.go)"},
		},
		"required": []string{"pattern"},
	}
}
func (t *GlobTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct{ Pattern string `json:"pattern"` }
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(req.Pattern)
	if err != nil {
		return nil, err
	}
	result, _ := json.Marshal(map[string]interface{}{
		"files": matches, "count": len(matches),
	})
	return result, nil
}
