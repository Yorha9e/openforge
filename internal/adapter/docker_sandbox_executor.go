package adapter

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"openforge/internal/shared/kernel"
)

// Security layer identifiers for the 5-layer Docker sandbox defense model (§6.2).
const (
	SecurityLayerTrivy     = iota + 1 // L1: Pre-flight vulnerability scan via Trivy
	SecurityLayerHardening             // L2: Read-only rootfs, cap-drop, cgroup limits
	SecurityLayerSeccomp               // L3: Seccomp syscall filter profile (JSON path)
	SecurityLayerNetwork               // L4: Network isolation + registry allowlist
	SecurityLayerValidate              // L5: Dangerous command pattern blocking
)

// Default sandbox security configuration values.
const (
	DefaultMemory     = "2g"                      // Memory limit (L2)
	DefaultCPU        = "2"                        // CPU limit (L2)
	DefaultPids       = 100                        // PIDs limit (L2)
	DefaultNetwork    = "none"                     // Network mode (L4)
	DefaultTrivyImage = "aquasec/trivy:latest"     // Trivy scanner image (L1)
)

// execLookPath is replaceable in tests.
var execLookPath = exec.LookPath

// DockerSandboxConfig holds Docker sandbox launch parameters.
type DockerSandboxConfig struct {
	Image       string
	MemoryMB    int
	CPUs        int
	MaxPids     int
	NetworkMode      string
	SeccompProfile   string   // Path to seccomp JSON profile (L3)
	RegistryWhitelist []string // Allowed image registries (L4)
	Timeout          time.Duration
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
		CPUs:        2,
		MaxPids:     100,
		NetworkMode:      DefaultNetwork,
		SeccompProfile:   "",     // opt-in; requires explicit profile path
		RegistryWhitelist: nil,   // opt-in; nil = all registries allowed
		Timeout:          30 * time.Second,
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
	if err := cfg.validateRegistry(); err != nil {
		return nil, err
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
	dockerCmd := e.buildDockerCmd(command, opts)

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

	dockerCmd := e.buildDockerCmd(command, opts)

	local := NewLocalShellExecutor(WithProfile(nil))
	return local.ExecuteStream(ctx, dockerCmd, kernel.ExecOptions{
		WorkDir:   opts.WorkDir,
		Timeout:   e.cfg.Timeout,
		MaxOutput: opts.MaxOutput,
	})
}

// buildDockerCmd constructs the docker run command string with proper escaping
// and all required flags.
func (e *DockerSandboxExecutor) buildDockerCmd(command string, opts kernel.ExecOptions) string {
	dockerCmd := fmt.Sprintf(
		"docker run --rm --init --read-only --cap-drop=ALL --memory=%dm --cpus=%d --pids-limit=%d --network=%s",
		e.cfg.MemoryMB, e.cfg.CPUs, e.cfg.MaxPids, e.cfg.NetworkMode,
	)

	if e.cfg.SeccompProfile != "" {
		dockerCmd = fmt.Sprintf("%s --security-opt=seccomp=%s", dockerCmd, e.cfg.SeccompProfile)
	}

	if opts.WorkDir != "" {
		dockerCmd = fmt.Sprintf("%s --workdir %s", dockerCmd, opts.WorkDir)
	}
	for k, v := range opts.Env {
		dockerCmd = fmt.Sprintf("%s -e %s=%s", dockerCmd, k, v)
	}

	dockerCmd = fmt.Sprintf("%s %s /bin/sh -c %s", dockerCmd, e.cfg.Image, shellQuote(command))
	return dockerCmd
}

// shellQuote wraps s in single quotes for safe passing to /bin/sh -c.
// Single quotes prevent ALL expansion: $(), backticks, variables, etc.
func shellQuote(s string) string {
	// Inside single quotes, EVERYTHING is literal.
	// The only character that can't appear inside single quotes is ' itself.
	// We handle it by ending the single quote, adding an escaped ', and resuming.
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

// validateRegistry checks the configured image against the registry whitelist (L4).
func (c DockerSandboxConfig) validateRegistry() error {
	if len(c.RegistryWhitelist) == 0 || c.Image == "" {
		return nil
	}
	for _, prefix := range c.RegistryWhitelist {
		if strings.HasPrefix(c.Image, prefix) {
			return nil
		}
	}
	return fmt.Errorf("image %q not in registry whitelist %v", c.Image, c.RegistryWhitelist)
}

// ScanImage runs a Trivy vulnerability scan against the given image (L1).
// Returns the raw scan report. Returns an error if Docker is unavailable.
func (e *DockerSandboxExecutor) ScanImage(ctx context.Context, image string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", DefaultTrivyImage, "image", "--quiet", image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("trivy scan: %w\n%s", err, string(out))
	}
	return string(out), nil
}

var _ kernel.CommandExecutor = (*DockerSandboxExecutor)(nil)
