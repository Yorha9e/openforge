package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"openforge/internal/shared/kernel"
)

type LocalShellExecutor struct {
	shell           string
	shellArgs       []string
	blockedCommands []string
	blockedPatterns []*regexp.Regexp
	allowedPaths    []string
}

type LocalShellOption func(*LocalShellExecutor)

func WithProfile(cfg interface{}) LocalShellOption {
	return func(e *LocalShellExecutor) {}
}

func NewLocalShellExecutor(opts ...LocalShellOption) *LocalShellExecutor {
	e := &LocalShellExecutor{
		shell:     detectShell(),
		shellArgs: detectShellArgs(),
		blockedCommands: []string{
			"rm", "dd", "mkfs", "sudo", "chmod", "chown",
			"mount", "umount", "fdisk", "parted",
		},
		blockedPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\|\s*sudo`),
			regexp.MustCompile(`;\s*sudo`),
			regexp.MustCompile(`>\s*/etc/`),
			regexp.MustCompile(`>\s*/usr/`),
			regexp.MustCompile(`curl.*\|\s*(ba)?sh`),
			regexp.MustCompile(`wget.*\|\s*(ba)?sh`),
			regexp.MustCompile(`:\(\)\s*\{.*:\|:&.*\};:`),
		},
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

func detectShell() string {
	if _, err := exec.LookPath("bash"); err == nil {
		return "/bin/bash"
	}
	return "powershell.exe"
}

func detectShellArgs() []string {
	if _, err := exec.LookPath("bash"); err == nil {
		return []string{"-c"}
	}
	return []string{"-NoProfile", "-NonInteractive", "-Command"}
}

func (e *LocalShellExecutor) Execute(ctx context.Context, command string, opts kernel.ExecOptions) (kernel.ExecOutput, error) {
	if err := e.Validate(ctx, command, opts); err != nil {
		return kernel.ExecOutput{}, err
	}
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.MaxOutput == 0 {
		opts.MaxOutput = 1 << 20
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.shell, append(e.shellArgs, command)...)
	cmd.Dir = opts.WorkDir
	if cmd.Dir == "" {
		cmd.Dir = "."
	}
	cmd.Env = buildEnv(opts.Env)

	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, max: opts.MaxOutput}
	cmd.Stderr = &limitedWriter{w: &stderr, max: opts.MaxOutput}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return kernel.ExecOutput{}, fmt.Errorf("command execution failed: %w", err)
		}
	}
	return kernel.ExecOutput{
		ExitCode: exitCode, Stdout: stdout.String(),
		Stderr: stderr.String(), Duration: time.Since(start),
	}, nil
}

type limitedWriter struct {
	w       io.Writer
	max     int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written >= lw.max {
		return len(p), nil
	}
	remaining := lw.max - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}

func (e *LocalShellExecutor) ExecuteStream(ctx context.Context, command string, opts kernel.ExecOptions) (<-chan kernel.ExecStreamChunk, error) {
	if err := e.Validate(ctx, command, opts); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, e.shell, append(e.shellArgs, command)...)
	cmd.Dir = opts.WorkDir
	if cmd.Dir == "" {
		cmd.Dir = "."
	}
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("command start failed: %w", err)
	}
	ch := make(chan kernel.ExecStreamChunk, 64)
	var wg sync.WaitGroup
	wg.Add(2)
	go streamLines(&wg, stdoutPipe, "stdout", ch)
	go streamLines(&wg, stderrPipe, "stderr", ch)
	go func() { wg.Wait(); cmd.Wait(); close(ch) }()
	return ch, nil
}

func (e *LocalShellExecutor) Validate(_ context.Context, command string, opts kernel.ExecOptions) error {
	if opts.ReadOnly {
		if err := e.assertReadOnly(command); err != nil {
			return fmt.Errorf("read-only violation: %w", err)
		}
	}
	baseCmd := strings.Fields(command)[0]
	for _, blocked := range e.blockedCommands {
		if baseCmd == blocked || strings.HasPrefix(baseCmd, blocked+".") {
			return fmt.Errorf("blocked command: %s", blocked)
		}
	}
	for _, pattern := range e.blockedPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("blocked pattern: %s", pattern.String())
		}
	}
	return nil
}

func (e *LocalShellExecutor) assertReadOnly(command string) error {
	readOnlyPrefixes := []string{
		"ls", "cat", "head", "tail", "grep", "find", "wc",
		"git status", "git log", "git diff", "git show",
		"echo", "which", "type", "pwd", "whoami", "date", "env",
		"node -v", "npm list", "go version",
	}
	trimmed := strings.TrimSpace(command)
	for _, ro := range readOnlyPrefixes {
		if strings.HasPrefix(trimmed, ro) {
			return nil
		}
	}
	return fmt.Errorf("command not in read-only allowlist: %s", command)
}

func buildEnv(extra map[string]string) []string {
	env := make([]string, 0, len(extra))
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

func streamLines(wg *sync.WaitGroup, r io.Reader, stream string, ch chan<- kernel.ExecStreamChunk) {
	defer wg.Done()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			ch <- kernel.ExecStreamChunk{Delta: string(buf[:n]), Stream: stream}
		}
		if err != nil {
			return
		}
	}
}
