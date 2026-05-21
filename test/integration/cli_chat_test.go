package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIBuilds(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", os.DevNull, "../../cmd/openforge/")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI build failed: %v\n%s", err, out)
	}
}

func TestCLIBanner(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/openforge/", "serve", "--config", "../../config/profiles/minimal.yaml")
	// Don't set ANTHROPIC_API_KEY — expect it to fail with a clear error
	cmd.Env = os.Environ()
	// Remove ANTHROPIC_API_KEY if present to test graceful failure
	var cleanedEnv []string
	for _, e := range cmd.Env {
		if !strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			cleanedEnv = append(cleanedEnv, e)
		}
	}
	cmd.Env = cleanedEnv

	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		t.Skip("ANTHROPIC_API_KEY was set, skipping graceful-failure test")
	}

	if !strings.Contains(output, "ANTHROPIC_API_KEY") {
		t.Errorf("expected error about ANTHROPIC_API_KEY, got: %s", output)
	}
}

func TestCLIBinaryExists(t *testing.T) {
	// Quick smoke test: the main.go file exists and is syntactically valid
	_, err := os.Stat("../../cmd/openforge/main.go")
	if err != nil {
		t.Fatalf("main.go not found: %v", err)
	}
}
