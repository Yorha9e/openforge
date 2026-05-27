package adapter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNginxLoadBalancer_AddBackend(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Add a backend
	err := lb.AddBackend(ctx, "web", "192.168.1.1:80")
	if err != nil {
		t.Fatalf("AddBackend failed: %v", err)
	}

	// Verify backend was added
	backends := lb.GetBackends("web")
	if len(backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(backends))
	}
	if backends[0] != "192.168.1.1:80" {
		t.Errorf("expected 192.168.1.1:80, got %s", backends[0])
	}
}

func TestNginxLoadBalancer_AddBackend_Duplicate(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Add same backend twice
	lb.AddBackend(ctx, "web", "192.168.1.1:80")
	lb.AddBackend(ctx, "web", "192.168.1.1:80")

	// Should still have only 1 backend
	backends := lb.GetBackends("web")
	if len(backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(backends))
	}
}

func TestNginxLoadBalancer_RemoveBackend(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Add and remove backend
	lb.AddBackend(ctx, "web", "192.168.1.1:80")
	err := lb.RemoveBackend(ctx, "web", "192.168.1.1:80")
	if err != nil {
		t.Fatalf("RemoveBackend failed: %v", err)
	}

	// Should have no backends
	backends := lb.GetBackends("web")
	if len(backends) != 0 {
		t.Fatalf("expected 0 backends, got %d", len(backends))
	}
}

func TestNginxLoadBalancer_RemoveBackend_NotFound(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Try to remove non-existent backend
	err := lb.RemoveBackend(ctx, "web", "192.168.1.1:80")
	if err == nil {
		t.Error("expected error when removing non-existent backend")
	}
}

func TestNginxLoadBalancer_HealthCheck(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Empty pool should be unhealthy
	healthy, err := lb.HealthCheck(ctx, "web")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	if healthy {
		t.Error("empty pool should be unhealthy")
	}

	// Add backend
	lb.AddBackend(ctx, "web", "192.168.1.1:80")

	// Pool with backends should be healthy
	healthy, err = lb.HealthCheck(ctx, "web")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	if !healthy {
		t.Error("pool with backends should be healthy")
	}
}

func TestNginxLoadBalancer_MultiplePools(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	ctx := context.Background()

	// Add to multiple pools
	lb.AddBackend(ctx, "web", "192.168.1.1:80")
	lb.AddBackend(ctx, "web", "192.168.1.2:80")
	lb.AddBackend(ctx, "api", "192.168.2.1:8080")

	// Verify pools
	webBackends := lb.GetBackends("web")
	if len(webBackends) != 2 {
		t.Errorf("expected 2 web backends, got %d", len(webBackends))
	}

	apiBackends := lb.GetBackends("api")
	if len(apiBackends) != 1 {
		t.Errorf("expected 1 api backend, got %d", len(apiBackends))
	}

	// List pools
	pools := lb.ListPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}

func TestNginxLoadBalancer_WriteConfig(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "nginx_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "upstream.conf")
	lb := NewNginxLoadBalancer(configPath)
	ctx := context.Background()

	// Add backends
	lb.AddBackend(ctx, "web", "192.168.1.1:80")
	lb.AddBackend(ctx, "web", "192.168.1.2:80")
	lb.AddBackend(ctx, "api", "192.168.2.1:8080")

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "upstream web") {
		t.Error("config should contain web upstream")
	}
	if !strings.Contains(contentStr, "upstream api") {
		t.Error("config should contain api upstream")
	}
	if !strings.Contains(contentStr, "192.168.1.1:80") {
		t.Error("config should contain web backend addresses")
	}
	if !strings.Contains(contentStr, "192.168.2.1:8080") {
		t.Error("config should contain api backend addresses")
	}
}

func TestNginxLoadBalancer_Close(t *testing.T) {
	lb := NewNginxLoadBalancer("")
	err := lb.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}