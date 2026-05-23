package tool

import (
	"context"
	"encoding/json"
	"time"

	agentport "openforge/internal/agent/port"
	"openforge/internal/shared/kernel"
)

type BashInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	WorkDir     string `json:"work_dir,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

type BashTool struct {
	executor kernel.CommandExecutor
}

func NewBashTool(executor kernel.CommandExecutor) *BashTool {
	return &BashTool{executor: executor}
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command. Use for npm install, git status, go test, etc." }
func (t *BashTool) InputSchema() []byte {
	s, _ := json.Marshal(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command":     map[string]string{"type": "string", "description": "Shell command to execute"},
			"description": map[string]string{"type": "string", "description": "Why this command is needed"},
			"work_dir":    map[string]string{"type": "string"},
			"timeout_ms":  map[string]string{"type": "integer"},
		},
		"required": []string{"command", "description"},
	})
	return s
}
func (t *BashTool) IsConcurrencySafe() bool { return false }
func (t *BashTool) IsReadOnly() bool        { return false }

func (t *BashTool) Execute(ctx context.Context, input BashInput) (kernel.ExecOutput, error) {
	return t.executor.Execute(ctx, input.Command, kernel.ExecOptions{
		WorkDir: input.WorkDir,
		Timeout: time.Duration(input.TimeoutMs) * time.Millisecond,
	})
}

func (t *BashTool) ExecuteStream(ctx context.Context, input BashInput) (<-chan agentport.StreamChunk[kernel.ExecOutput], error) {
	ch, err := t.executor.ExecuteStream(ctx, input.Command, kernel.ExecOptions{
		WorkDir: input.WorkDir,
		Timeout: time.Duration(input.TimeoutMs) * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}

	out := make(chan agentport.StreamChunk[kernel.ExecOutput], 64)
	go func() {
		defer close(out)
		for chunk := range ch {
			out <- agentport.StreamChunk[kernel.ExecOutput]{
				Value: kernel.ExecOutput{Stdout: chunk.Delta, Stderr: chunk.Stream},
			}
		}
	}()
	return out, nil
}

var _ agentport.Tool[BashInput, kernel.ExecOutput] = (*BashTool)(nil)
var _ agentport.StreamingTool[BashInput, kernel.ExecOutput] = (*BashTool)(nil)
