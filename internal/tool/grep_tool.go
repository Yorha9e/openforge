package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search for a pattern in file contents" }
func (t *GrepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]string{"type": "string", "description": "Pattern to search for (substring match)"},
			"path":    map[string]string{"type": "string", "description": "File or directory to search in"},
		},
		"required": []string{"pattern", "path"},
	}
}
func (t *GrepTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, err
	}
	var matches []string
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, req.Pattern) {
			matches = append(matches, fmt.Sprintf("%d:%s", i+1, line))
		}
	}
	result, _ := json.Marshal(map[string]interface{}{
		"matches": matches, "count": len(matches),
	})
	return result, nil
}
