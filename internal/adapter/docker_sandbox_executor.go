package adapter

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"openforge/internal/shared/kernel"
)

// execLookPath is replaceable in tests.
var execLookPath = exec.LookPath

// DockerSandboxConfig holds Docker sandbox launch parameters.
type DockerSandboxConfig struct {
	Image       string
	MemoryMB    int
	CPUShares   int
	MaxPids     int
	NetworkMode string
	Timeout     time.Duration
}

// dangerousPatterns matches commands that are hard-blocked.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/`),
	regexp.MustCompile(`sudo\b`),
	regexp.MustCompile(`\bdd\b.*if=`),
	regexp.MustCompile(`\bmkfs\.`),
	regexp.MustCompile(`curl.*\|.*bash`),
	regexp.MustCompile(`wget.*\|.*sh`),
	regexp.MustCompile(`>/dev/sd[a-z]`),
	regexp.MustCompile(`:(){ :|:& };:`), // fork bomb
}

func defaultDockerSandboxConfig() DockerSandboxConfig {
	return DockerSandboxConfig{
		Image:       "openforge/sandbox-node:latest",
		MemoryMB:    2048,
		CPUShares:   2,
		MaxPids:     100,
		NetworkMode: "none",
		Timeout:     30 * time.Second,
	}
}

// DockerSandboxExecutor implements CommandExecutor via Docker containers.
// Run 'docker run --rm --read-only --cap-drop=ALL ...' for each command.
type DockerSandboxExecutor struct {
	cfg DockerSandboxConfig
}

// NewDockerSandboxExecutor creates a new DockerSandboxExecutor.
// If cfg.Image is empty, default configuration is used.
// Returns an error if the docker CLI or daemon is not available.
func NewDockerSandboxExecutor(cfg DockerSandboxConfig) (*DockerSandboxExecutor, error) {
	if cfg.Image == "" {
		cfg = defaultDockerSandboxConfig()
	}
	if _, err := execLookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker CLI not found: %w", err)
	}
	// Verify the Docker daemon is reachable.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err := info.Run(); err != nil {
		return nil, fmt.Errorf("docker daemon not reachable: %w", err)
	}
	return &DockerSandboxExecutor{cfg: cfg}, nil
}

// Execute runs a command inside a Docker sandbox container.
func (e *DockerSandboxExecutor) Execute(ctx context.Context, command string, opts kernel.ExecOptions) (kernel.ExecOutput, error) {
	if err := e.Validate(ctx, command, opts); err != nil {
		return kernel.ExecOutput{}, err
	}

	start := time.Now()
	// Phase 4 MVP: delegate to LocalShellExecutor wrapped with docker CLI.
	// Post-Phase 4: use Docker SDK (github.com/docker/docker/client) for direct API access.
	dockerCmd := fmt.Sprintf(
		"docker run --rm --read-only --cap-drop=ALL --memory=%dm --cpus=%d --pids-limit=%d --network=%s %s /bin/sh -c %q",
		e.cfg.MemoryMB, e.cfg.CPUShares, e.cfg.MaxPids, e.cfg.NetworkMode,
		e.cfg.Image, command,
	)

	local := NewLocalShellExecutor(WithProfile(nil))
	out, err := local.Execute(ctx, dockerCmd, kernel.ExecOptions{
		WorkDir:   opts.WorkDir,
		Timeout:   e.cfg.Timeout,
		MaxOutput: opts.MaxOutput,
	})
	out.Duration = time.Since(start)
	return out, err
}

// ExecuteStream runs a command inside a Docker sandbox container with streaming output.
func (e *DockerSandboxExecutor) ExecuteStream(ctx context.Context, command string, opts kernel.ExecOptions) (<-chan kernel.ExecStreamChunk, error) {
	if err := e.Validate(ctx, command, opts); err != nil {
		return nil, err
	}

	dockerCmd := fmt.Sprintf(
		"docker run --rm --read-only --cap-drop=ALL --memory=%dm --cpus=%d --pids-limit=%d --network=%s %s /bin/sh -c %q",
		e.cfg.MemoryMB, e.cfg.CPUShares, e.cfg.MaxPids, e.cfg.NetworkMode,
		e.cfg.Image, command,
	)

	local := NewLocalShellExecutor(WithProfile(nil))
	return local.ExecuteStream(ctx, dockerCmd, kernel.ExecOptions{
		WorkDir:   opts.WorkDir,
		Timeout:   e.cfg.Timeout,
		MaxOutput: opts.MaxOutput,
	})
}

// Validate checks whether a command is safe to execute in the sandbox.
func (e *DockerSandboxExecutor) Validate(ctx context.Context, command string, opts kernel.ExecOptions) error {
	for _, p := range dangerousPatterns {
		if p.MatchString(command) {
			return fmt.Errorf("dangerous command blocked: %q matches %s", command, p.String())
		}
	}
	return nil
}
