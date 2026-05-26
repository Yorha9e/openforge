package tool

import (
	"context"
	"encoding/json"
	"time"

	"openforge/internal/shared/kernel"
)

// BashToolAdapter adapts the generic BashTool to the simple tool.Tool interface.
type BashToolAdapter struct {
	Executor kernel.CommandExecutor
}

func (t *BashToolAdapter) Name() string { return "bash" }
func (t *BashToolAdapter) Description() string {
	return "Execute a shell command. Use for npm install, git status, go test, etc."
}
func (t *BashToolAdapter) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command":     map[string]string{"type": "string", "description": "Shell command to execute"},
			"description": map[string]string{"type": "string", "description": "Why this command is needed"},
			"work_dir":    map[string]string{"type": "string", "description": "Working directory"},
			"timeout_ms":  map[string]string{"type": "integer", "description": "Timeout in milliseconds"},
		},
		"required": []string{"command", "description"},
	}
}

func (t *BashToolAdapter) Run(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Command     string `json:"command"`
		Description string `json:"description"`
		WorkDir     string `json:"work_dir,omitempty"`
		TimeoutMs   int    `json:"timeout_ms,omitempty"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}

	timeout := 60 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	output, err := t.Executor.Execute(ctx, req.Command, kernel.ExecOptions{
		WorkDir: req.WorkDir,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(map[string]interface{}{
		"stdout":    output.Stdout,
		"stderr":    output.Stderr,
		"exit_code": output.ExitCode,
		"duration":  output.Duration.Milliseconds(),
	})
	return result, nil
}
