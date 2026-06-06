package adapter

import (
	"testing"
	"time"

	"openforge/internal/pipeline/domain"
)

func TestIsValidStage(t *testing.T) {
	tests := []struct {
		stage string
		want  bool
	}{
		{"clarify", true},
		{"decompose", true},
		{"impl", true},
		{"test", true},
		{"deploy", true},
		{"verify", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := domain.IsValidStage(tt.stage); got != tt.want {
			t.Errorf("IsValidStage(%q) = %v, want %v", tt.stage, got, tt.want)
		}
	}
}

func TestIsValidStageStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"pending", true},
		{"running", true},
		{"awaiting_gate", true},
		{"passed", true},
		{"failed", true},
		{"skipped", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := domain.IsValidStageStatus(tt.status); got != tt.want {
			t.Errorf("IsValidStageStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestPQStringArray(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{nil, "{}"},
		{[]string{}, "{}"},
		{[]string{"a"}, `{"a"}`},
		{[]string{"a", "b", "c"}, `{"a","b","c"}`},
	}
	for _, tt := range tests {
		if got := pqStringArray(tt.input); got != tt.want {
			t.Errorf("pqStringArray(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseStringArray(t *testing.T) {
	tests := []struct {
		input []byte
		want  int // expected length
	}{
		{nil, 0},
		{[]byte("{}"), 0},
		{[]byte(`{"a"}`), 1},
		{[]byte(`{"a","b","c"}`), 3},
	}
	for _, tt := range tests {
		got := parseStringArray(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseStringArray(%s) returned %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestStageStruct(t *testing.T) {
	now := time.Now()
	s := &domain.Stage{
		ID:         "stage-1",
		PipelineID: "pipe-1",
		Type:       "impl",
		Status:     "running",
		Summary:    "Implementation in progress",
		StartedAt:  &now,
	}

	if s.Type != "impl" {
		t.Errorf("expected Type=impl, got %s", s.Type)
	}
	if s.Status != "running" {
		t.Errorf("expected Status=running, got %s", s.Status)
	}
	if s.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}
