package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SearchFileTool finds files by name pattern.
type SearchFileTool struct{}

func (t *SearchFileTool) Name() string { return "search_file" }
func (t *SearchFileTool) Description() string {
	return "Search for files by name pattern (supports wildcards)"
}
func (t *SearchFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern":   map[string]string{"type": "string", "description": "File pattern (e.g., *.go, *.ts)"},
			"path":      map[string]string{"type": "string", "description": "Directory to search in (default: current)"},
			"recursive": map[string]string{"type": "boolean", "description": "Search recursively (default: true)"},
		},
		"required": []string{"pattern"},
	}
}

func (t *SearchFileTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Pattern   string `json:"pattern"`
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}

	if req.Path == "" {
		req.Path = "."
	}
	// Default to recursive
	if !req.Recursive {
		req.Recursive = true
	}

	var matches []string

	if req.Recursive {
		err := filepath.Walk(req.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				// Skip hidden directories and node_modules
				if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if matchPattern(path, req.Pattern) {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk: %w", err)
		}
	} else {
		entries, err := os.ReadDir(req.Path)
		if err != nil {
			return nil, fmt.Errorf("read dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(req.Path, entry.Name())
			if matchPattern(path, req.Pattern) {
				matches = append(matches, path)
			}
		}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"files": matches,
		"count": len(matches),
	})
	return result, nil
}

func matchPattern(path, pattern string) bool {
	name := filepath.Base(path)
	matched, _ := filepath.Match(pattern, name)
	return matched
}
