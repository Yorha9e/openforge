package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// DeleteFileTool deletes a file at the specified path.
type DeleteFileTool struct{}

func (t *DeleteFileTool) Name() string { return "delete_file" }
func (t *DeleteFileTool) Description() string {
	return "Delete a file at the specified path"
}
func (t *DeleteFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]string{"type": "string", "description": "File path to delete"},
		},
		"required": []string{"path"},
	}
}

func (t *DeleteFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", req.Path)
	}

	if err := os.Remove(req.Path); err != nil {
		return nil, fmt.Errorf("delete file: %w", err)
	}

	result, _ := json.Marshal(map[string]string{"status": "ok"})
	return result, nil
}
