package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditFileTool replaces old_str with new_str in a file.
type EditFileTool struct{}

func (t *EditFileTool) Name() string { return "edit_file" }
func (t *EditFileTool) Description() string {
	return "Replace text in a file. Use for targeted edits."
}
func (t *EditFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]string{"type": "string", "description": "File path to edit"},
			"old_str": map[string]string{"type": "string", "description": "Text to replace"},
			"new_str": map[string]string{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"path", "old_str", "new_str"},
	}
}

func (t *EditFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path   string `json:"path"`
		OldStr string `json:"old_str"`
		NewStr string `json:"new_str"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	text := string(content)
	if !strings.Contains(text, req.OldStr) {
		return nil, fmt.Errorf("old_str not found in file")
	}

	// Replace first occurrence
	newContent := strings.Replace(text, req.OldStr, req.NewStr, 1)
	if err := os.WriteFile(req.Path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	result, _ := json.Marshal(map[string]string{"status": "ok"})
	return result, nil
}
