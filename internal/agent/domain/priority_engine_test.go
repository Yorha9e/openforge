package domain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyUpdateNoSkills(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSkillLoader([]string{dir})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	upe := NewUnifiedPriorityEngine(sl, nil)
	err = upe.RunDailyUpdate()
	if err != nil {
		t.Fatalf("RunDailyUpdate: %v", err)
	}
}

func TestCalculatePriority(t *testing.T) {
	upe := NewUnifiedPriorityEngine(nil, nil)
	cfg := upe.config.Skill.Priority

	skill := Skill{
		BasePriority:    80,
		IsLatest:        true,
		CurrentPriority: 80,
	}

	newP := upe.calculatePriority(skill, nil, cfg)
	if newP != 80 {
		t.Errorf("latest skill should have base priority, got %.1f", newP)
	}

	// Old version — should decay
	skill.IsLatest = false
	skill.Version = "0.9.0"
	latestMap := map[string]time.Time{
		skill.Name: time.Now().Add(-10 * 24 * time.Hour), // latest published 10d ago
	}
	newP = upe.calculatePriority(skill, latestMap, cfg)
	if newP >= 80 {
		t.Errorf("old version should decay, got %.1f", newP)
	}
}

func TestVersionDecayRate(t *testing.T) {
	upe := NewUnifiedPriorityEngine(nil, nil) // uses defaults

	cfg := upe.config.Skill.Priority
	skill := Skill{
		Name:             "test-skill",
		BasePriority:     70,
		IsLatest:         false,
		CurrentPriority:  70,
	}

	// 30 days after latest → should be significantly decayed
	latestMap := map[string]time.Time{
		"test-skill": time.Now().Add(-30 * 24 * time.Hour),
	}
	newP := upe.calculatePriority(skill, latestMap, cfg)
	// e^(-0.1 * 30) ≈ 0.05, so priority should be around 70*0.05 = 3.5
	if newP > 70*cfg.MinVersionFactor*2 {
		t.Errorf("30d old version should be near min factor, got %.2f", newP)
	}
}

func TestMinVersionFactor(t *testing.T) {
	upe := NewUnifiedPriorityEngine(nil, nil)

	cfg := upe.config.Skill.Priority
	skill := Skill{
		Name:             "very-old",
		BasePriority:     70,
		IsLatest:         false,
		CurrentPriority:  70,
	}

	// 365 days after latest → should be clamped to min factor
	latestMap := map[string]time.Time{
		"very-old": time.Now().Add(-365 * 24 * time.Hour),
	}
	newP := upe.calculatePriority(skill, latestMap, cfg)

	minExpected := 70 * cfg.MinVersionFactor
	if newP < minExpected-0.01 {
		t.Errorf("priority should not go below min: %.2f < %.2f", newP, minExpected)
	}
}

func TestWritePriorityUpdates(t *testing.T) {
	dir := t.TempDir()

	cfgContent := `skills:
  - name: test-skill
    version: "1.0.0"
    file: test-skill.md
    base_priority: 80
    current_priority: 80.0
    enabled: true
    is_latest: true
    deprecated: false
    published_at: "2026-05-24T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(dir, "skill_config.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	upe := NewUnifiedPriorityEngine(nil, nil)
	updates := []priorityUpdate{
		{name: "test-skill", version: "1.0.0", oldPriority: 80, newPriority: 15, shouldDeprecate: true},
	}
	err := upe.writePriorityUpdates(dir, updates)
	if err != nil {
		t.Fatalf("writePriorityUpdates: %v", err)
	}

	// Verify the file was updated
	data, err := os.ReadFile(filepath.Join(dir, "skill_config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !contains(content, "15") {
		t.Errorf("expected updated priority 15 in output: %s", content)
	}
}

func TestDefaultSkillEngineConfig(t *testing.T) {
	cfg := DefaultSkillEngineConfig()
	if cfg.Skill.MaxInject != 5 {
		t.Errorf("expected MaxInject=5, got %d", cfg.Skill.MaxInject)
	}
	if cfg.Skill.Priority.DefaultBase != 70 {
		t.Errorf("expected DefaultBase=70, got %.0f", cfg.Skill.Priority.DefaultBase)
	}
	if cfg.Skill.Priority.MinVersionFactor != 0.05 {
		t.Errorf("expected MinVersionFactor=0.05, got %.2f", cfg.Skill.Priority.MinVersionFactor)
	}
}

// TestPriorityEngine_LearningFactor_ReflectsSuccessRate verifies that the
// LearningFactor (T10) correctly computes a 0.5..1.5 range from the project's
// trajectory success rate (TrajectoryRecord with empty FailureCodes counts as
// success; non-empty counts as failure).
func TestPriorityEngine_LearningFactor_ReflectsSuccessRate(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()
	// p-good: 3 success + 1 failure → rate 0.75 → factor 1.25
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-good", PipelineID: "g1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-good", PipelineID: "g2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-good", PipelineID: "g3"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-good", PipelineID: "g4", FailureCodes: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	// p-bad: 2 failures → rate 0.0 → factor 0.5
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-bad", PipelineID: "b1", FailureCodes: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(ctx, TrajectoryRecord{ProjectID: "p-bad", PipelineID: "b2", FailureCodes: []string{"y"}}); err != nil {
		t.Fatal(err)
	}

	upe := NewUnifiedPriorityEngine(nil, nil)
	upe.SetTrajectoryStore(store)

	gotGood := upe.LearningFactor(ctx, "p-good")
	if gotGood < 1.24 || gotGood > 1.26 {
		t.Errorf("p-good LearningFactor: got %.3f, want ≈1.25", gotGood)
	}
	gotBad := upe.LearningFactor(ctx, "p-bad")
	if gotBad < 0.49 || gotBad > 0.51 {
		t.Errorf("p-bad LearningFactor: got %.3f, want ≈0.50", gotBad)
	}
}

// TestPriorityEngine_LearningFactor_NoTrajectories verifies an empty store
// returns the neutral factor 1.0 (no data → no adjustment).
func TestPriorityEngine_LearningFactor_NoTrajectories(t *testing.T) {
	store := NewMemTrajectoryStore()
	upe := NewUnifiedPriorityEngine(nil, nil)
	upe.SetTrajectoryStore(store)

	got := upe.LearningFactor(context.Background(), "unknown-project")
	if got != 1.0 {
		t.Errorf("empty store: got %.3f, want 1.0", got)
	}
}

// TestPriorityEngine_LearningFactor_NilStore verifies a nil trajectory store
// returns the neutral factor 1.0 (defensive default).
func TestPriorityEngine_LearningFactor_NilStore(t *testing.T) {
	upe := NewUnifiedPriorityEngine(nil, nil)
	// Intentionally do not call SetTrajectoryStore — upe.trajStore remains nil.
	got := upe.LearningFactor(context.Background(), "any-project")
	if got != 1.0 {
		t.Errorf("nil store: got %.3f, want 1.0", got)
	}
}
