package domain

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createTempSkillDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Write skill_config.yaml
	cfgContent := `skills:
  - name: test-skill
    version: "1.0.0"
    file: test-skill.md
    base_priority: 80
    current_priority: 85.0
    enabled: true
    is_latest: true
    deprecated: false
    published_at: "2026-05-24T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(dir, "skill_config.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a seed skill
	skillContent := `---
name: test-skill
version: "1.0.0"
stages: [impl, test]
complexity: [L2, L3]
permission: [auto, default]
keywords: [testing, go, backend]
triggers:
  file_patterns: ["*.go", "*_test.go"]
  user_intent: [add test, fix test]
base_priority: 80
---

# Test Skill

## Prompt
This is a test prompt for Go backend testing.

## Workflow
1. Check existing test patterns
2. Create test file following conventions
3. Run tests and verify
`
	if err := os.WriteFile(filepath.Join(dir, "test-skill.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestNewSkillLoader(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	snapshot := sl.GetSnapshot()
	if len(snapshot.skills) == 0 {
		t.Fatal("no skills loaded")
	}
	if snapshot.skills[0].Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", snapshot.skills[0].Name)
	}
	if snapshot.skills[0].Prompt == "" {
		t.Error("expected non-empty prompt from body parsing")
	}
	if len(snapshot.skills[0].Workflow) == 0 {
		t.Error("expected workflow steps")
	}
	if snapshot.skills[0].CurrentPriority != 85.0 {
		t.Errorf("expected CurrentPriority=85.0 (from config.yaml), got %.1f", snapshot.skills[0].CurrentPriority)
	}
}

func TestSkillLoaderMatch(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	req := MatchRequest{
		Stage:       "impl",
		UserMessage: "add unit tests for the backend Go code",
		TokenBudget: 100000,
		MaxInject:   5,
	}
	skills := sl.Match(req)
	if len(skills) == 0 {
		t.Fatal("no skills matched")
	}
	if skills[0].Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", skills[0].Name)
	}
}

func TestSkillLoaderMatchStageFilter(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	// Stage not in skill's stages list
	req := MatchRequest{
		Stage:       "clarify",
		UserMessage: "test",
		TokenBudget: 100000,
		MaxInject:   5,
	}
	skills := sl.Match(req)
	if len(skills) != 0 {
		t.Error("expected no matches for stage 'clarify'")
	}
}

func TestSkillLoaderMatchByName(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	skill, err := sl.MatchByName("test-skill")
	if err != nil {
		t.Fatalf("MatchByName: %v", err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", skill.Name)
	}

	// Non-existent skill
	_, err = sl.MatchByName("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestSkillLoaderForceSkill(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	// ForceSkill should bypass stage filter
	req := MatchRequest{
		Stage:      "clarify", // test-skill doesn't have clarify
		ForceSkill: "test-skill",
		TokenBudget: 100000,
		MaxInject:  5,
	}
	skills := sl.Match(req)
	if len(skills) == 0 {
		t.Fatal("ForceSkill should bypass stage filter")
	}
	if skills[0].Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", skills[0].Name)
	}
}

func TestSkillLoaderReload(t *testing.T) {
	dir := createTempSkillDir(t)
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	if err := sl.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	snapshot := sl.GetSnapshot()
	if len(snapshot.skills) == 0 {
		t.Fatal("reload lost skills")
	}
}

func TestSkillLoaderEmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		// Should not error; degrades to empty snapshot
		t.Fatalf("NewSkillLoader should degrade: %v", err)
	}
	defer sl.Stop()

	skills := sl.GetAllSkills()
	if len(skills) != 0 {
		t.Error("expected empty skills for nonexistent dir")
	}
}

func TestSkillLoaderDeprecatedFilter(t *testing.T) {
	dir := t.TempDir()

	cfgContent := `skills:
  - name: old-skill
    version: "0.9.0"
    file: old-skill.md
    base_priority: 70
    current_priority: 0.1
    enabled: true
    is_latest: false
    deprecated: true
    published_at: "2025-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(dir, "skill_config.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	skillContent := `---
name: old-skill
version: "0.9.0"
stages: [impl]
keywords: [deprecated]
base_priority: 70
---

# Old Skill

## Prompt
This skill is deprecated.
`
	if err := os.WriteFile(filepath.Join(dir, "old-skill.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	req := MatchRequest{Stage: "impl", UserMessage: "test", TokenBudget: 100000, MaxInject: 5}
	skills := sl.Match(req)
	if len(skills) != 0 {
		t.Error("deprecated skill should not match")
	}
}

func TestConduitSeedSkills(t *testing.T) {
	// Test that the seed skills can be parsed without error
	tmpDir := t.TempDir()

	backendContent := `---
name: conduit-backend
version: "1.0.0"
stages: [impl, test, deploy]
complexity: [L2, L3, L4]
permission: [auto, default]
keywords: [express, typescript, api, route, middleware, model, backend, conduit]
triggers:
  file_patterns: ["backend/src/**/*.ts", "*.ts"]
  user_intent: [add api, create route, add endpoint, backend change, fix backend]
base_priority: 80
---

# Conduit Backend Pattern

## Prompt
You are working on the Conduit RealWorld backend, an Express + TypeScript application.
Follow these conventions:
- Routes are defined in backend/src/routes/ using Express Router
- Models use TypeORM or Prisma in backend/src/models/
- Use zod for request validation
- API responses follow the { status, data, errors } envelope pattern

## Workflow
1. Read existing route files to understand the API pattern
2. Read the Prisma schema for data shape
3. Create or modify route handler following conventions
4. Add or update middleware if needed
5. Write test
`
	frontendContent := `---
name: conduit-frontend
version: "1.0.0"
stages: [impl, test]
complexity: [L2, L3, L4]
permission: [auto, default]
keywords: [react, typescript, component, hooks, state, form, ui, frontend, conduit]
triggers:
  file_patterns: ["frontend/src/**/*.tsx", "*.tsx", "*.ts"]
  user_intent: [create page, add component, ui change, frontend change, fix frontend]
base_priority: 80
---

# Conduit Frontend Pattern

## Prompt
You are working on the Conduit RealWorld frontend, a React + TypeScript application.
Follow these conventions:
- Components are in frontend/src/components/ or frontend/src/features/
- Use React Context + useReducer for state management
- Prefer editing existing files over creating new ones

## Workflow
1. Check existing components for reusable patterns
2. Read the API client for available endpoints
3. Create or modify components following existing conventions
`
	cfgContent := `skills:
  - name: conduit-backend
    version: "1.0.0"
    file: conduit-backend.md
    base_priority: 80
    current_priority: 80.0
    enabled: true
    is_latest: true
    deprecated: false
    published_at: "2026-05-24T00:00:00Z"
  - name: conduit-frontend
    version: "1.0.0"
    file: conduit-frontend.md
    base_priority: 80
    current_priority: 80.0
    enabled: true
    is_latest: true
    deprecated: false
    published_at: "2026-05-24T00:00:00Z"
`

	if err := os.WriteFile(filepath.Join(tmpDir, "skill_config.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "conduit-backend.md"), []byte(backendContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "conduit-frontend.md"), []byte(frontendContent), 0644); err != nil {
		t.Fatal(err)
	}

	sl, err := NewSkillLoader([]string{tmpDir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	all := sl.GetAllSkills()
	if len(all) != 2 {
		t.Errorf("expected 2 skills, got %d", len(all))
	}

	// Backend match
	backendReq := MatchRequest{Stage: "impl", UserMessage: "add an api endpoint for articles", TokenBudget: 100000, MaxInject: 5}
	backend := sl.Match(backendReq)
	if len(backend) == 0 {
		t.Error("expected backend skill to match API request")
	}

	// Frontend match
	frontendReq := MatchRequest{Stage: "impl", UserMessage: "create a new page component for user profile", TokenBudget: 100000, MaxInject: 5}
	frontend := sl.Match(frontendReq)
	if len(frontend) == 0 {
		t.Error("expected frontend skill to match UI request")
	}
}

func init() {
	time.Local = time.UTC
}
