package domain

import (
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
