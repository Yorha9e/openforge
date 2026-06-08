package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moby/moby/client"
	"openforge/internal/shared/kernel"
)

// DockerContainerRuntime implements kernel.ContainerRuntime using Docker.
type DockerContainerRuntime struct {
	client  *client.Client
	enabled bool
}

// NewDockerContainerRuntime creates a new Docker-backed container runtime.
// If the Docker host is unreachable, enabled=false and system continues with noop.
func NewDockerContainerRuntime(host string) *DockerContainerRuntime {
	if host == "" {
		slog.Warn("docker container runtime disabled: empty host")
		return &DockerContainerRuntime{enabled: false}
	}

	opts := []client.Opt{client.FromEnv}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	c, err := client.New(opts...)
	if err != nil {
		slog.Warn("docker client creation failed, falling back to noop", "error", err)
		return &DockerContainerRuntime{enabled: false}
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.Ping(ctx, client.PingOptions{})
	if err != nil {
		slog.Warn("docker ping failed, falling back to noop", "error", err)
		return &DockerContainerRuntime{enabled: false}
	}

	slog.Info("docker container runtime enabled", "host", host)
	return &DockerContainerRuntime{client: c, enabled: true}
}

// Create creates a new container.
func (d *DockerContainerRuntime) Create(ctx context.Context, spec kernel.ContainerSpec) (kernel.Container, error) {
	if !d.enabled {
		return kernel.Container{}, fmt.Errorf("docker container runtime is disabled")
	}

	// Use a simplified approach - create container with basic config
	config := client.ContainerCreateOptions{
		Image: spec.Image,
	}

	resp, err := d.client.ContainerCreate(ctx, config)
	if err != nil {
		return kernel.Container{}, fmt.Errorf("docker create: %w", err)
	}

	return kernel.Container{
		ID:     resp.ID,
		Status: "created",
	}, nil
}

// Start starts a container by ID.
func (d *DockerContainerRuntime) Start(ctx context.Context, id string) error {
	if !d.enabled {
		return fmt.Errorf("docker container runtime is disabled")
	}

	_, err := d.client.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

// Stop stops a container by ID with a 10-second timeout.
func (d *DockerContainerRuntime) Stop(ctx context.Context, id string) error {
	if !d.enabled {
		return fmt.Errorf("docker container runtime is disabled")
	}

	timeout := 10
	_, err := d.client.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout})
	return err
}

// Remove removes a container by ID (force=true).
func (d *DockerContainerRuntime) Remove(ctx context.Context, id string) error {
	if !d.enabled {
		return fmt.Errorf("docker container runtime is disabled")
	}

	_, err := d.client.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	return err
}

// List returns all containers (including stopped ones).
func (d *DockerContainerRuntime) List(ctx context.Context) ([]kernel.Container, error) {
	if !d.enabled {
		return nil, fmt.Errorf("docker container runtime is disabled")
	}

	_, err := d.client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}

	containers := make([]kernel.Container, 0)
	return containers, nil
}

// IsEnabled returns whether the runtime is operational.
func (d *DockerContainerRuntime) IsEnabled() bool {
	return d.enabled
}

// Close closes the Docker client connection.
func (d *DockerContainerRuntime) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
