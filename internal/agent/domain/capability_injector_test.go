package domain

import (
	"context"
	"testing"
)

func TestNewCapabilityInjector(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	ci := NewCapabilityInjector(sl, &HardcodedToolRegistry{})
	if ci == nil {
		t.Fatal("expected non-nil CapabilityInjector")
	}
}

func TestCapabilityInjectorInject(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	ci := NewCapabilityInjector(sl, &HardcodedToolRegistry{})

	req := CapabilityRequest{
		PipelineID:     "pipe-1",
		ProjectID:      "proj-A",
		Stage:          "impl",
		UserMessage:    "add unit tests for the Go backend",
		PermissionMode: "auto",
		TokenBudget:    100000,
	}

	result, err := ci.Inject(context.Background(), req)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have matched test-skill (keywords: testing, go, backend)
	if len(result.Skills) == 0 {
		t.Error("expected at least one skill match")
	}
	// Should have tools for "impl" stage
	if len(result.Tools) == 0 {
		t.Error("expected at least one tool for impl stage")
	}
}

func TestCapabilityInjectorTokenBudgetZero(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	ci := NewCapabilityInjector(sl, &HardcodedToolRegistry{})

	req := CapabilityRequest{
		Stage:       "impl",
		UserMessage: "add tests for backend in go",
		TokenBudget: 0, // zero budget triggers pending_user
	}

	result, err := ci.Inject(context.Background(), req)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if !result.PendingUser {
		t.Error("expected PendingUser when TokenBudget=0")
	}
	if result.Message == "" {
		t.Error("expected a prompt message")
	}
}

func TestCapabilityInjectorForceSkill(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	ci := NewCapabilityInjector(sl, &HardcodedToolRegistry{})

	req := CapabilityRequest{
		Stage:       "clarify", // test-skill doesn't apply to clarify
		UserMessage: "anything",
		TokenBudget: 100000,
		ForceSkill:  "test-skill",
	}

	result, err := ci.Inject(context.Background(), req)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if len(result.Skills) != 1 {
		t.Fatalf("expected 1 forced skill, got %d", len(result.Skills))
	}
	if result.Skills[0].Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", result.Skills[0].Name)
	}
}

func TestCapabilityInjectorNoSkillLoader(t *testing.T) {
	ci := NewCapabilityInjector(nil, &HardcodedToolRegistry{})

	req := CapabilityRequest{
		Stage:       "impl",
		UserMessage: "test",
		TokenBudget: 100000,
	}

	result, err := ci.Inject(context.Background(), req)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if len(result.Skills) != 0 {
		t.Error("expected no skills when SkillLoader is nil")
	}
	if len(result.Tools) == 0 {
		t.Error("expected tools even without skills")
	}
}

func TestCapabilityInjectorBuildXML(t *testing.T) {
	ci := NewCapabilityInjector(nil, nil)
	result := &CapabilityResult{
		Skills: []SkillInjectionRecord{
			{Name: "test-skill", Version: "1.0.0", Source: "global", TriggerScore: 25.0},
		},
		Tools: []ToolInjectionRecord{
			{Name: "read_file", Description: "Read file", ReadOnly: true},
		},
	}

	xml := ci.BuildCapabilityXML(result)
	if xml == "" {
		t.Error("expected non-empty XML output")
	}
	// Verify XML contains skill name
	if !contains(xml, "test-skill") {
		t.Error("XML should contain skill name")
	}
	if !contains(xml, "read_file") {
		t.Error("XML should contain tool name")
	}
}

func TestHardcodedToolRegistry(t *testing.T) {
	reg := &HardcodedToolRegistry{}
	tools, err := reg.SearchTools(context.Background(), "implement", 5)
	if err != nil {
		t.Fatalf("SearchTools: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("expected tools for implement stage")
	}
	if len(tools) > 5 {
		t.Errorf("expected at most 5, got %d", len(tools))
	}
}

func TestHardcodedToolRegistryUnknownStage(t *testing.T) {
	reg := &HardcodedToolRegistry{}
	tools, err := reg.SearchTools(context.Background(), "unknown_stage", 10)
	if err != nil {
		t.Fatalf("SearchTools: %v", err)
	}
	if tools != nil {
		t.Error("expected nil for unknown stage")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
