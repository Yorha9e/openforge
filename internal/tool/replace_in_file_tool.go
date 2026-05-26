package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReplaceInFileTool performs exact string replacements in an existing file.
type ReplaceInFileTool struct{}

func (t *ReplaceInFileTool) Name() string { return "replace_in_file" }
func (t *ReplaceInFileTool) Description() string {
	return "Perform exact string replacements in an existing file"
}
func (t *ReplaceInFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]string{"type": "string", "description": "File path to modify"},
			"old_str": map[string]string{"type": "string", "description": "Exact text to replace (must be unique)"},
			"new_str": map[string]string{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"path", "old_str", "new_str"},
	}
}

func (t *ReplaceInFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path   string `json:"path"`
		OldStr string `json:"old_str"`
		NewStr string `json:"new_str"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}

	if req.OldStr == "" {
		return nil, fmt.Errorf("old_str cannot be empty")
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	text := string(content)
	count := strings.Count(text, req.OldStr)
	if count == 0 {
		return nil, fmt.Errorf("old_str not found in file")
	}
	if count > 1 {
		return nil, fmt.Errorf("old_str found %d times, must be unique", count)
	}

	newContent := strings.Replace(text, req.OldStr, req.NewStr, 1)
	if err := os.WriteFile(req.Path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":       "ok",
		"replacements": 1,
	})
	return result, nil
}
