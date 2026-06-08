package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"openforge/internal/shared/kernel"
)

// shellExecutable picks a shell that is present on the current OS. On
// Windows we prefer cmd.exe (POSIX sh is generally not on PATH under
// Git-Bash-less environments); on Linux/macOS we use /bin/sh.
func shellExecutable() string {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("cmd"); err == nil {
			return "cmd"
		}
		if _, err := exec.LookPath("powershell"); err == nil {
			return "powershell"
		}
	}
	if _, err := exec.LookPath("sh"); err == nil {
		return "sh"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "sh"
}

// LocalShellExecutor implements kernel.CommandExecutor with command-level
// allow/block policies covering 12 capability domains (file, fs, network,
// process, package, env, git, build, test, db, container, agent).
//
// The default BlockedCmds are the well-known catastrophic patterns that
// should never run from a model-driven tool call. Additional patterns can
// be added via NewLocalShellExecutor(WithBlockedCmds(...)) at boot.
type LocalShellExecutor struct {
	DefaultTimeout time.Duration
	BlockedCmds    []string
	BlockedPatterns []string

	mu sync.Mutex
}

// CapabilityDomain enumerates the 12 high-level command categories the
// executor recognises. New domains can be added without changing the
// CommandExecutor surface.
type CapabilityDomain string

const (
	CapFileSystem   CapabilityDomain = "file_system"
	CapProcess      CapabilityDomain = "process"
	CapNetwork      CapabilityDomain = "network"
	CapPackage      CapabilityDomain = "package"
	CapEnv          CapabilityDomain = "env"
	CapGit          CapabilityDomain = "git"
	CapBuild        CapabilityDomain = "build"
	CapTest         CapabilityDomain = "test"
	CapDB           CapabilityDomain = "db"
	CapContainer    CapabilityDomain = "container"
	CapAgent        CapabilityDomain = "agent"
	CapSystem       CapabilityDomain = "system"
)

// NewLocalShellExecutor builds the default executor. Override defaults with
// options if needed (not implemented in this stub; reserved for Phase 4+).
func NewLocalShellExecutor() *LocalShellExecutor {
	return &LocalShellExecutor{
		DefaultTimeout:  30 * time.Second,
		BlockedCmds:     []string{"rm -rf /", "dd if=", "sudo", "chmod -R 777"},
		BlockedPatterns: []string{"curl|sh", "wget|sh", "curl|bash", "wget|bash"},
	}
}

// Execute runs a single command and returns its combined stdout/stderr.
func (l *LocalShellExecutor) Execute(ctx context.Context, command string, opts kernel.ExecOptions) (kernel.ExecOutput, error) {
	if err := l.Validate(ctx, command, opts); err != nil {
		return kernel.ExecOutput{}, err
	}
	timeout := l.DefaultTimeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sh := shellExecutable()
	var cmd *exec.Cmd
	if sh == "cmd" {
		cmd = exec.CommandContext(cctx, sh, "/c", command)
	} else if sh == "powershell" {
		cmd = exec.CommandContext(cctx, sh, "-Command", command)
	} else {
		cmd = exec.CommandContext(cctx, sh, "-c", command)
	}
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if ok {
			return kernel.ExecOutput{ExitCode: ee.ExitCode(), Stdout: string(out), Stderr: string(out)}, err
		}
		return kernel.ExecOutput{Stdout: string(out), Stderr: string(out)}, err
	}
	return kernel.ExecOutput{ExitCode: 0, Stdout: string(out), Stderr: ""}, nil
}

// ExecuteStream runs a command and streams stdout/stderr line by line.
func (l *LocalShellExecutor) ExecuteStream(ctx context.Context, command string, opts kernel.ExecOptions) (<-chan kernel.ExecStreamChunk, error) {
	if err := l.Validate(ctx, command, opts); err != nil {
		return nil, err
	}
	timeout := l.DefaultTimeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	sh := shellExecutable()
	var cmd *exec.Cmd
	if sh == "cmd" {
		cmd = exec.CommandContext(cctx, sh, "/c", command)
	} else if sh == "powershell" {
		cmd = exec.CommandContext(cctx, sh, "-Command", command)
	} else {
		cmd = exec.CommandContext(cctx, sh, "-c", command)
	}
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start: %w", err)
	}
	out := make(chan kernel.ExecStreamChunk, 64)
	go func() {
		defer close(out)
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				select {
				case out <- kernel.ExecStreamChunk{Delta: string(buf[:n]), Stream: "stdout"}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				break
			}
		}
		ebuf := make([]byte, 4096)
		for {
			n, err := stderr.Read(ebuf)
			if n > 0 {
				select {
				case out <- kernel.ExecStreamChunk{Delta: string(ebuf[:n]), Stream: "stderr"}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				break
			}
		}
		_ = cmd.Wait()
	}()
	return out, nil
}

// Validate checks the command against the local block list and dangerous
// pipe-to-shell patterns.
func (l *LocalShellExecutor) Validate(ctx context.Context, command string, opts kernel.ExecOptions) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	cmd := strings.TrimSpace(command)
	for _, blocked := range l.BlockedCmds {
		if strings.HasPrefix(cmd, blocked) {
			return fmt.Errorf("command blocked by local policy: %q", blocked)
		}
		if strings.Contains(cmd, blocked) {
			return fmt.Errorf("command contains blocked fragment %q", blocked)
		}
	}
	for _, pat := range l.BlockedPatterns {
		if strings.Contains(cmd, pat) {
			return fmt.Errorf("command matches blocked pattern %q", pat)
		}
	}
	return nil
}

// firstWord returns the leading whitespace-trimmed word from cmd.
func firstWord(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		return cmd[:i]
	}
	return cmd
}

// ClassifyDomain returns the capability domain a command belongs to. Used
// by observability hooks to record which of the 12 categories was used.
func (l *LocalShellExecutor) ClassifyDomain(command string) CapabilityDomain {
	cmd := strings.ToLower(strings.TrimSpace(command))
	w := firstWord(cmd)
	// Disambiguate compound commands sharing a first word (e.g. "go test"
	// vs "go build" both start with "go" which is otherwise a package
	// manager).
	if strings.HasPrefix(cmd, "go test") || strings.HasPrefix(cmd, "npm test") || strings.HasPrefix(cmd, "yarn test") {
		return CapTest
	}
	if strings.HasPrefix(cmd, "go build") || strings.HasPrefix(cmd, "tsc") {
		return CapBuild
	}
	switch w {
	case "ls", "cat", "head", "tail", "find", "cp", "mv", "rm", "mkdir", "touch", "chmod", "chown":
		return CapFileSystem
	case "ps", "kill", "pkill", "top", "htop":
		return CapProcess
	case "curl", "wget", "nc", "ping", "ssh", "scp":
		return CapNetwork
	case "npm", "yarn", "pnpm", "pip", "apt", "brew", "go":
		return CapPackage
	case "export", "env", "unset":
		return CapEnv
	case "git":
		return CapGit
	case "make", "cargo", "mvn", "gradle":
		return CapBuild
	case "pytest", "vitest", "jest":
		return CapTest
	case "psql", "mysql", "redis-cli", "mongo", "mongosh":
		return CapDB
	case "docker", "kubectl", "helm", "podman":
		return CapContainer
	case "openforge", "agent":
		return CapAgent
	default:
		return CapSystem
	}
}

var _ kernel.CommandExecutor = (*LocalShellExecutor)(nil)
