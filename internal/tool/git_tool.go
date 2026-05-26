package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GitStatusTool shows the working tree status.
type GitStatusTool struct{}

func (t *GitStatusTool) Name() string { return "git_status" }
func (t *GitStatusTool) Description() string {
	return "Show the working tree status (git status)"
}
func (t *GitStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *GitStatusTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status: %w\n%s", err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []map[string]string
	for _, line := range lines {
		if line == "" {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[2:])
		files = append(files, map[string]string{
			"status": status,
			"path":   path,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"files": files,
		"count": len(files),
	})
	return result, nil
}

// GitDiffTool shows changes between commits, commit and working tree, etc.
type GitDiffTool struct{}

func (t *GitDiffTool) Name() string { return "git_diff" }
func (t *GitDiffTool) Description() string {
	return "Show changes in the working tree (git diff)"
}
func (t *GitDiffTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]string{"type": "string", "description": "File or directory to diff (optional)"},
		},
	}
}

func (t *GitDiffTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Path string `json:"path"`
	}
	json.Unmarshal(input, &req)

	args := []string{"diff"}
	if req.Path != "" {
		args = append(args, req.Path)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w\n%s", err, string(output))
	}

	result, _ := json.Marshal(map[string]string{"diff": string(output)})
	return result, nil
}

// GitLogTool shows commit logs.
type GitLogTool struct{}

func (t *GitLogTool) Name() string { return "git_log" }
func (t *GitLogTool) Description() string {
	return "Show commit logs (last 10 commits)"
}
func (t *GitLogTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"count": map[string]string{"type": "integer", "description": "Number of commits to show (default 10)"},
		},
	}
}

func (t *GitLogTool) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Count int `json:"count"`
	}
	json.Unmarshal(input, &req)
	if req.Count <= 0 {
		req.Count = 10
	}

	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("-%d", req.Count), "--oneline")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log: %w\n%s", err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []map[string]string
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, map[string]string{
				"hash":    parts[0],
				"message": parts[1],
			})
		}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"commits": commits,
		"count":   len(commits),
	})
	return result, nil
}
