package tool

import (
	"context"
	"encoding/json"
	"os"
)

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file" }
func (t *WriteFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]string{"type": "string", "description": "File path to write"},
			"content": map[string]string{"type": "string", "description": "Content to write"},
		},
		"required": []string{"path", "content"},
	}
}
func (t *WriteFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "ok"})
}
