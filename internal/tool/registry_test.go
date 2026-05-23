package tool

import (
	"context"
	"testing"

	"openforge/internal/agent/port"
)

func TestRegistry_RegisterAndSearch(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	reg.Register(ctx, port.ToolInfo{
		Name: "read_file", Description: "Read contents of a file by path",
	})
	reg.Register(ctx, port.ToolInfo{
		Name: "write_file", Description: "Write content to a file",
	})
	reg.Register(ctx, port.ToolInfo{
		Name: "bash", Description: "Execute a shell command",
	})

	matches, err := reg.Search(ctx, "read a file", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match")
	}
	if matches[0].Name != "read_file" {
		t.Errorf("top match = %q, want read_file", matches[0].Name)
	}
	if matches[0].Score <= 0 {
		t.Error("score should be > 0")
	}
}

func TestRegistry_RunTool(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterTool(&EchoTool{})

	result, err := reg.Run(context.Background(), port.ToolCall{
		ToolName: "echo",
		Input:    []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	expected := `{"message":"hello"}`
	if string(result.Output) != expected {
		t.Errorf("output = %s, want %s", string(result.Output), expected)
	}
}

func TestRegistry_SearchToolsBackwardCompat(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	reg.Register(ctx, port.ToolInfo{
		Name: "bash", Description: "Execute a shell command",
	})

	matches, err := reg.SearchTools(ctx, "bash", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match via SearchTools")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	reg.Register(ctx, port.ToolInfo{Name: "t1", Description: "First tool"})
	reg.Register(ctx, port.ToolInfo{Name: "t2", Description: "Second tool"})

	infos, err := reg.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Errorf("List count = %d, want 2", len(infos))
	}
}

func TestRegistry_RunUnknownTool(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Run(context.Background(), port.ToolCall{
		ToolName: "nonexistent",
		Input:    []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
