package domain

import (
	"context"
	"fmt"
	"testing"

	"openforge/internal/agent/service"
)

func TestCoordinator_SpawnAgent(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	agent, err := coord.Spawn(ctx, "agent-1", "proj-1", "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if agent.ID != "agent-1" {
		t.Errorf("agent ID = %q, want agent-1", agent.ID)
	}
	if coord.AgentCount() != 1 {
		t.Errorf("agent count = %d, want 1", coord.AgentCount())
	}
}

func TestCoordinator_Delegate(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "pm", "proj-1", "pm", "")
	coord.Spawn(ctx, "dev-1", "proj-1", "dev", "pm")

	err := coord.Delegate(ctx, "pm", "dev-1", service.Message{
		From: "pm",
		To:   "dev-1",
		Body: []byte(`{"task":"review code"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_Broadcast(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "a1", "p1", "dev", "")
	coord.Spawn(ctx, "a2", "p1", "dev", "")

	err := coord.Broadcast(ctx, "pm", service.Message{
		From: "pm",
		Body: []byte(`{"event":"status_update"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_SpawnLimit(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := coord.Spawn(ctx, fmt.Sprintf("a%d", i), "p1", "dev", "")
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	_, err := coord.Spawn(ctx, "a6", "p1", "dev", "")
	if err == nil {
		t.Fatal("expected spawn limit error")
	}
}

func TestCoordinator_Terminate(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "temp", "p1", "dev", "")
	if err := coord.Terminate(ctx, "temp"); err != nil {
		t.Fatal(err)
	}
	if coord.AgentCount() != 0 {
		t.Errorf("agent count = %d, want 0 after terminate", coord.AgentCount())
	}
}

func TestCoordinator_DelegateToUnknown(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	err := coord.Delegate(ctx, "pm", "nonexistent", service.Message{
		From: "pm",
		Body: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestCoordinator_ListAgents(t *testing.T) {
	coord := NewCoordinator(nil, nil)
	ctx := context.Background()

	coord.Spawn(ctx, "a1", "p1", "pm", "")
	coord.Spawn(ctx, "a2", "p1", "dev", "a1")

	agents := coord.ListAgents()
	if len(agents) != 2 {
		t.Errorf("ListAgents count = %d, want 2", len(agents))
	}
}
