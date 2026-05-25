package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestToolRegistry_Partition(t *testing.T) {
	reg := ToolRegistry{
		"read_file":  {Name: "read_file", IsConcurrencySafe: true, Timeout: 30 * time.Second},
		"write_file": {Name: "write_file", IsConcurrencySafe: false, Timeout: 30 * time.Second},
		"grep":       {Name: "grep", IsConcurrencySafe: true, Timeout: 30 * time.Second},
		"bash":       {Name: "bash", IsConcurrencySafe: false, Timeout: 60 * time.Second},
	}

	calls := []ToolCallParsed{
		{ID: "1", Name: "read_file"},
		{ID: "2", Name: "grep"},
		{ID: "3", Name: "write_file"},
		{ID: "4", Name: "bash"},
	}

	groups := reg.partition(calls)

	// Expected: [read_file+grep parallel], [write_file serial], [bash serial]
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if !groups[0].Parallel || len(groups[0].Tools) != 2 {
		t.Errorf("group[0] expected parallel with 2 tools, got parallel=%v len=%d", groups[0].Parallel, len(groups[0].Tools))
	}
	if groups[1].Parallel || len(groups[1].Tools) != 1 {
		t.Errorf("group[1] expected serial with 1 tool")
	}
	if groups[2].Parallel || len(groups[2].Tools) != 1 {
		t.Errorf("group[2] expected serial with 1 tool")
	}
}

func TestToolRegistry_Partition_AllParallel(t *testing.T) {
	reg := ToolRegistry{
		"read_file": {Name: "read_file", IsConcurrencySafe: true, Timeout: 30 * time.Second},
		"grep":      {Name: "grep", IsConcurrencySafe: true, Timeout: 30 * time.Second},
		"glob":      {Name: "glob", IsConcurrencySafe: true, Timeout: 30 * time.Second},
	}

	calls := []ToolCallParsed{
		{ID: "1", Name: "read_file"},
		{ID: "2", Name: "grep"},
		{ID: "3", Name: "glob"},
	}

	groups := reg.partition(calls)
	if len(groups) != 1 || !groups[0].Parallel || len(groups[0].Tools) != 3 {
		t.Errorf("expected 1 parallel group with 3 tools, got %d groups", len(groups))
	}
}

func TestToolRegistry_ExecuteOne_Success(t *testing.T) {
	reg := ToolRegistry{
		"test_tool": {
			Name: "test_tool", IsConcurrencySafe: true, Timeout: 5 * time.Second,
			Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
				return "done", nil
			},
		},
	}

	policy := ToolErrorPolicy{MaxRetries: 3, RetryDelay: 10 * time.Millisecond, BackoffFactor: 2.0}
	result := reg.executeOne(context.Background(), ToolCallParsed{Name: "test_tool"}, policy)

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Output)
	}
	if result.Tool != "test_tool" {
		t.Errorf("expected tool=test_tool, got %s", result.Tool)
	}
}

func TestToolRegistry_ExecuteOne_RetryThenSuccess(t *testing.T) {
	attempts := 0
	reg := ToolRegistry{
		"flaky_tool": {
			Name: "flaky_tool", IsConcurrencySafe: true, Timeout: 5 * time.Second,
			Executor: func(ctx context.Context, args map[string]interface{}) (string, error) {
				attempts++
				if attempts < 3 {
					return "", errors.New("temporary error: API timeout")
				}
				return "finally worked", nil
			},
		},
	}

	policy := ToolErrorPolicy{MaxRetries: 3, RetryDelay: 10 * time.Millisecond, BackoffFactor: 2.0}
	result := reg.executeOne(context.Background(), ToolCallParsed{Name: "flaky_tool"}, policy)

	if result.Status != "success" {
		t.Errorf("expected success after retries, got %s", result.Status)
	}
	if result.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", result.Retries)
	}
}

func TestToolRegistry_ExecuteOne_Unregistered(t *testing.T) {
	reg := ToolRegistry{}
	policy := ToolErrorPolicy{MaxRetries: 3, RetryDelay: 10 * time.Millisecond, BackoffFactor: 2.0}
	result := reg.executeOne(context.Background(), ToolCallParsed{Name: "nonexistent"}, policy)

	if result.Status != "error" {
		t.Errorf("expected error for unregistered tool, got %s", result.Status)
	}
}

func TestFormatToolResultXML(t *testing.T) {
	r := FormattedToolResult{
		Tool: "bash", Status: "success", Output: "Build completed",
		DurationMs: 1234,
	}

	xml := formatToolResultXML(r)

	if !containsStr(xml, "tool=\"bash\"") {
		t.Errorf("missing tool name: %s", xml)
	}
	if !containsStr(xml, "status=\"success\"") {
		t.Errorf("missing status: %s", xml)
	}
	if !containsStr(xml, "Build completed") {
		t.Errorf("missing output: %s", xml)
	}
	if !containsStr(xml, "duration_ms=\"1234\"") {
		t.Errorf("missing duration: %s", xml)
	}
}

func TestFormatToolResultXML_WithError(t *testing.T) {
	r := FormattedToolResult{
		Tool: "bash", Status: "error", Output: "command not found",
		ErrorCode: "MODEL_HALLUCINATION", Retries: 3, DurationMs: 5000,
	}

	xml := formatToolResultXML(r)

	if !containsStr(xml, "error_code=\"MODEL_HALLUCINATION\"") {
		t.Errorf("missing error_code: %s", xml)
	}
	if !containsStr(xml, "retries=\"3\"") {
		t.Errorf("missing retries: %s", xml)
	}
}
