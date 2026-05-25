package domain

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultToolRegistry builds a ToolRegistry with the basic tools for the current platform.
func DefaultToolRegistry() ToolRegistry {
	reg := make(ToolRegistry)

	// read_file tool
	reg["read_file"] = ToolMeta{
		Name:              "read_file",
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path required", nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			return string(data), nil
		},
	}

	// write_file tool
	reg["write_file"] = ToolMeta{
		Name:              "write_file",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "error: " + err.Error(), nil
			}
			return "wrote " + path, nil
		},
	}

	// bash tool
	reg["bash"] = ToolMeta{
		Name:              "bash",
		IsConcurrencySafe: false,
		Timeout:           60 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "error: command required", nil
			}
			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out) + "\nerror: " + err.Error(), nil
			}
			return string(out), nil
		},
	}

	// grep tool
	reg["grep"] = ToolMeta{
		Name:              "grep",
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pattern, _ := args["pattern"].(string)
			path, _ := args["path"].(string)
			if pattern == "" {
				return "error: pattern required", nil
			}
			cmd := exec.CommandContext(ctx, "grep", "-rn", pattern, path)
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// glob tool
	reg["glob"] = ToolMeta{
		Name:              "glob",
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				pattern = "*"
			}
			cmd := exec.CommandContext(ctx, "bash", "-c", "ls "+pattern+" 2>/dev/null || echo 'no matches'")
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// ls tool
	reg["ls"] = ToolMeta{
		Name:              "ls",
		IsConcurrencySafe: true,
		Timeout:           10 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			cmd := exec.CommandContext(ctx, "ls", "-la", path)
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// search_content (grep alias for Anthropic compatibility)
	reg["search_content"] = reg["grep"]
	reg["list_files"] = reg["ls"]

	return reg
}

// FormatToolsForPrompt generates the tool descriptions XML for the system prompt.
func FormatToolsForPrompt(reg ToolRegistry) string {
	var b strings.Builder
	b.WriteString("<tools>\n")
	for name, meta := range reg {
		safety := "false"
		if meta.IsConcurrencySafe {
			safety = "true"
		}
		b.WriteString("  <tool>\n")
		b.WriteString("    <name>" + name + "</name>\n")
		b.WriteString("    <readonly>" + safety + "</readonly>\n")
		b.WriteString("  </tool>\n")
	}
	b.WriteString("</tools>")
	return b.String()
}
