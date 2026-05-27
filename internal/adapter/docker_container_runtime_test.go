package adapter

import (
	"context"
	"os"
	"testing"
)

func dockerAvailable(t *testing.T) {
	t.Helper()
	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping docker integration test")
	}
}

func TestDockerContainerRuntime_Disabled_EmptyHost(t *testing.T) {
	runtime := NewDockerContainerRuntime("")
	if runtime.IsEnabled() {
		t.Error("should be disabled when host is empty")
	}

	ctx := context.Background()
	_, err := runtime.Create(ctx, ContainerSpec{Image: "alpine"})
	if err == nil {
		t.Error("Create should return error when disabled")
	}

	err = runtime.Start(ctx, "test")
	if err == nil {
		t.Error("Start should return error when disabled")
	}

	err = runtime.Stop(ctx, "test")
	if err == nil {
		t.Error("Stop should return error when disabled")
	}

	err = runtime.Remove(ctx, "test")
	if err == nil {
		t.Error("Remove should return error when disabled")
	}

	_, err = runtime.List(ctx)
	if err == nil {
		t.Error("List should return error when disabled")
	}
}

func TestDockerContainerRuntime_Disabled_BadHost(t *testing.T) {
	runtime := NewDockerContainerRuntime("tcp://localhost:1") // unreachable
	if runtime.IsEnabled() {
		t.Error("should be disabled when host is unreachable")
	}
}

func TestDockerContainerRuntime_CreateStartStop(t *testing.T) {
	dockerAvailable(t)

	host := os.Getenv("DOCKER_HOST")
	runtime := NewDockerContainerRuntime(host)
	if !runtime.IsEnabled() {
		t.Fatal("runtime should be enabled")
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create a simple container
	container, err := runtime.Create(ctx, ContainerSpec{
		Image: "alpine:latest",
		Cmd:   []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Start
	err = runtime.Start(ctx, container.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop
	err = runtime.Stop(ctx, container.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Remove
	err = runtime.Remove(ctx, container.ID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
}

func TestDockerContainerRuntime_List(t *testing.T) {
	dockerAvailable(t)

	host := os.Getenv("DOCKER_HOST")
	runtime := NewDockerContainerRuntime(host)
	if !runtime.IsEnabled() {
		t.Fatal("runtime should be enabled")
	}
	defer runtime.Close()

	ctx := context.Background()
	containers, err := runtime.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	t.Logf("Found %d containers", len(containers))
}
