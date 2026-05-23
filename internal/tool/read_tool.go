package tool

import (
	"context"
	"encoding/json"
	"os"
)

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read contents of a file" }
func (t *ReadFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]string{"type": "string", "description": "File path to read"},
		},
		"required": []string{"path"},
	}
}
func (t *ReadFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct{ Path string `json:"path"` }
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, err
	}
	result, _ := json.Marshal(map[string]string{"content": string(content)})
	return result, nil
}
