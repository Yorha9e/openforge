package domain

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultToolRegistry builds a ToolRegistry with the basic tools for the current platform.
func DefaultToolRegistry() ToolRegistry {
	return DefaultToolRegistryWithWorkDir("")
}

// DefaultToolRegistryWithWorkDir builds a ToolRegistry with working directory support.
// If workDir is empty, tools default to the process working directory.
func DefaultToolRegistryWithWorkDir(workDir string) ToolRegistry {
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
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
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
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			// Ensure parent directory exists
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "error: " + err.Error(), nil
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "error: " + err.Error(), nil
			}
			return "wrote " + path, nil
		},
	}

	// bash tool (cross-platform)
	reg["bash"] = ToolMeta{
		Name:              "bash",
		IsConcurrencySafe: false,
		Timeout:           60 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "error: command required", nil
			}
			shell, shellArgs := shellCommand(command)
			cmd := exec.CommandContext(ctx, shell, shellArgs...)
			if workDir != "" {
				cmd.Dir = workDir
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out) + "\nerror: " + err.Error(), nil
			}
			return string(out), nil
		},
	}

	// grep tool (cross-platform)
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
			if workDir != "" && path != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			if path == "" {
				path = "."
			}
			var cmd *exec.Cmd
			if isWindows() {
				// Use findstr on Windows
				cmd = exec.CommandContext(ctx, "findstr", "/S", "/N", "/R", pattern, filepath.Join(path, "*"))
			} else {
				cmd = exec.CommandContext(ctx, "grep", "-rn", pattern, path)
			}
			if workDir != "" {
				cmd.Dir = workDir
			}
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// glob tool (cross-platform)
	reg["glob"] = ToolMeta{
		Name:              "glob",
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				pattern = "*"
			}
			var cmd *exec.Cmd
			if isWindows() {
				// Use dir on Windows with wildcard
				cmd = exec.CommandContext(ctx, "cmd.exe", "/C", "dir", "/B", pattern)
			} else {
				cmd = exec.CommandContext(ctx, "bash", "-c", "ls "+pattern+" 2>/dev/null || echo 'no matches'")
			}
			if workDir != "" {
				cmd.Dir = workDir
			}
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// ls tool (cross-platform)
	reg["ls"] = ToolMeta{
		Name:              "ls",
		IsConcurrencySafe: true,
		Timeout:           10 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			var cmd *exec.Cmd
			if isWindows() {
				// Use dir on Windows
				cmd = exec.CommandContext(ctx, "cmd.exe", "/C", "dir", path)
			} else {
				cmd = exec.CommandContext(ctx, "ls", "-la", path)
			}
			if workDir != "" {
				cmd.Dir = workDir
			}
			out, _ := cmd.CombinedOutput()
			return string(out), nil
		},
	}

	// append_file tool
	reg["append_file"] = ToolMeta{
		Name:              "append_file",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			defer f.Close()
			if _, err := f.WriteString(content); err != nil {
				return "error: " + err.Error(), nil
			}
			return "appended to " + path, nil
		},
	}

	// file_exists tool
	reg["file_exists"] = ToolMeta{
		Name:              "file_exists",
		IsConcurrencySafe: true,
		Timeout:           5 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					return "false", nil
				}
				return "error: " + err.Error(), nil
			}
			if info.IsDir() {
				return "directory", nil
			}
			return "file", nil
		},
	}

	// mkdir tool
	reg["mkdir"] = ToolMeta{
		Name:              "mkdir",
		IsConcurrencySafe: false,
		Timeout:           10 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			if err := os.MkdirAll(path, 0755); err != nil {
				return "error: " + err.Error(), nil
			}
			return "created " + path, nil
		},
	}

	// delete_file tool
	reg["delete_file"] = ToolMeta{
		Name:              "delete_file",
		IsConcurrencySafe: false,
		Timeout:           10 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			if err := os.Remove(path); err != nil {
				return "error: " + err.Error(), nil
			}
			return "deleted " + path, nil
		},
	}

	// pwd tool
	reg["pwd"] = ToolMeta{
		Name:              "pwd",
		IsConcurrencySafe: true,
		Timeout:           5 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			if workDir != "" {
				return workDir, nil
			}
			dir, err := os.Getwd()
			if err != nil {
				return "error: " + err.Error(), nil
			}
			return dir, nil
		},
	}

	// edit_file tool - find and replace content in file
	reg["edit_file"] = ToolMeta{
		Name:              "edit_file",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			oldStr, _ := args["old_str"].(string)
			newStr, _ := args["new_str"].(string)
			if path == "" || oldStr == "" {
				return "error: path and old_str required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			content := string(data)
			if !strings.Contains(content, oldStr) {
				return "error: old_str not found in file", nil
			}
			newContent := strings.Replace(content, oldStr, newStr, 1)
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return "error: " + err.Error(), nil
			}
			return "edited " + path, nil
		},
	}

	// copy_file tool - copy a file
	reg["copy_file"] = ToolMeta{
		Name:              "copy_file",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			src, _ := args["src"].(string)
			dst, _ := args["dst"].(string)
			if src == "" || dst == "" {
				return "error: src and dst required", nil
			}
			if workDir != "" && !isAbs(src) {
				src = filepath.Join(workDir, src)
			}
			if workDir != "" && !isAbs(dst) {
				dst = filepath.Join(workDir, dst)
			}
			// Ensure destination directory exists
			dir := filepath.Dir(dst)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "error: " + err.Error(), nil
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return "error: " + err.Error(), nil
			}
			return "copied " + src + " -> " + dst, nil
		},
	}

	// move_file tool - move/rename a file
	reg["move_file"] = ToolMeta{
		Name:              "move_file",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			src, _ := args["src"].(string)
			dst, _ := args["dst"].(string)
			if src == "" || dst == "" {
				return "error: src and dst required", nil
			}
			if workDir != "" && !isAbs(src) {
				src = filepath.Join(workDir, src)
			}
			if workDir != "" && !isAbs(dst) {
				dst = filepath.Join(workDir, dst)
			}
			// Ensure destination directory exists
			dir := filepath.Dir(dst)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "error: " + err.Error(), nil
			}
			if err := os.Rename(src, dst); err != nil {
				return "error: " + err.Error(), nil
			}
			return "moved " + src + " -> " + dst, nil
		},
	}

	// read_lines tool - read specific line range from file
	reg["read_lines"] = ToolMeta{
		Name:              "read_lines",
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			startLine, _ := args["start"].(float64)
			endLine, _ := args["end"].(float64)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			file, err := os.Open(path)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 0
			var result strings.Builder
			for scanner.Scan() {
				lineNum++
				if startLine > 0 && float64(lineNum) < startLine {
					continue
				}
				if endLine > 0 && float64(lineNum) > endLine {
					break
				}
				result.WriteString(fmt.Sprintf("%d: %s\n", lineNum, scanner.Text()))
			}
			if err := scanner.Err(); err != nil {
				return "error: " + err.Error(), nil
			}
			return result.String(), nil
		},
	}

	// insert_at_line tool - insert content at specific line
	reg["insert_at_line"] = ToolMeta{
		Name:              "insert_at_line",
		IsConcurrencySafe: false,
		Timeout:           30 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			lineNum, _ := args["line"].(float64)
			content, _ := args["content"].(string)
			if path == "" || lineNum <= 0 {
				return "error: path and line (>0) required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			file, err := os.Open(path)
			if err != nil {
				return "error: " + err.Error(), nil
			}

			var lines []string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			file.Close()

			if err := scanner.Err(); err != nil {
				return "error: " + err.Error(), nil
			}

			// Insert content at specified line
			insertIdx := int(lineNum) - 1
			if insertIdx > len(lines) {
				insertIdx = len(lines)
			}
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:insertIdx]...)
			newLines = append(newLines, content)
			newLines = append(newLines, lines[insertIdx:]...)

			if err := os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
				return "error: " + err.Error(), nil
			}
			return fmt.Sprintf("inserted at line %d in %s", int(lineNum), path), nil
		},
	}

	// file_info tool - get file metadata
	reg["file_info"] = ToolMeta{
		Name:              "file_info",
		IsConcurrencySafe: true,
		Timeout:           5 * time.Second,
		Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "error: path required", nil
			}
			if workDir != "" && !isAbs(path) {
				path = filepath.Join(workDir, path)
			}
			info, err := os.Stat(path)
			if err != nil {
				return "error: " + err.Error(), nil
			}
			return fmt.Sprintf("name: %s\nsize: %d bytes\nisDir: %t\nmodTime: %s",
				info.Name(), info.Size(), info.IsDir(), info.ModTime().Format(time.RFC3339)), nil
		},
	}

	return reg
}

// isAbs checks if a path is absolute (platform-aware).
func isAbs(path string) bool {
	if len(path) == 0 {
		return false
	}
	// Unix absolute path
	if path[0] == '/' {
		return true
	}
	// Windows UNC path (e.g., \\server\share)
	if len(path) >= 2 && ((path[0] == '\\' && path[1] == '\\') || (path[0] == '/' && path[1] == '/')) {
		return true
	}
	// Windows absolute path (e.g., C:\ or C:/)
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return false
}

// isWindows returns true if running on Windows.
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// shellCommand returns the appropriate shell command for the current platform.
func shellCommand(script string) (string, []string) {
	if isWindows() {
		// Try PowerShell first, fall back to cmd
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			return "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", script}
		}
		return "cmd.exe", []string{"/C", script}
	}
	// Unix: try bash, fall back to sh
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash", []string{"-c", script}
	}
	return "sh", []string{"-c", script}
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
