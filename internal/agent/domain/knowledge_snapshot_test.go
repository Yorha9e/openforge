package domain

import (
	"context"
	"encoding/json"
	"testing"
)

func TestKnowledgeSnapshotCreateAndGetLatest(t *testing.T) {
	ctx := context.Background()
	store := NewMemKnowledgeSnapshotStore()

	snap := &KnowledgeSnapshot{
		ProjectID:          "proj-a",
		SnapshotData:       json.RawMessage(`{"key":"value"}`),
		HealthBaseline:     0.95,
		CodeAcceptanceRate: 0.85,
	}

	if err := store.Create(ctx, snap); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if snap.ID != "ks-1" {
		t.Errorf("expected ID ks-1, got %s", snap.ID)
	}
	if snap.Version != 1 {
		t.Errorf("expected Version 1, got %d", snap.Version)
	}

	// second snapshot auto-increments version
	snap2 := &KnowledgeSnapshot{
		ProjectID:          "proj-a",
		SnapshotData:       json.RawMessage(`{"key":"value2"}`),
		HealthBaseline:     0.98,
		CodeAcceptanceRate: 0.90,
	}
	if err := store.Create(ctx, snap2); err != nil {
		t.Fatalf("Create second failed: %v", err)
	}
	if snap2.Version != 2 {
		t.Errorf("expected Version 2, got %d", snap2.Version)
	}

	latest, err := store.GetLatest(ctx, "proj-a")
	if err != nil {
		t.Fatalf("GetLatest failed: %v", err)
	}
	if latest.Version != 2 {
		t.Errorf("expected latest version 2, got %d", latest.Version)
	}
	if latest.CodeAcceptanceRate != 0.90 {
		t.Errorf("expected CodeAcceptanceRate 0.90, got %f", latest.CodeAcceptanceRate)
	}
}

func TestKnowledgeSnapshotListByProject(t *testing.T) {
	ctx := context.Background()
	store := NewMemKnowledgeSnapshotStore()

	for i := 0; i < 3; i++ {
		snap := &KnowledgeSnapshot{
			ProjectID:          "proj-b",
			SnapshotData:       json.RawMessage(`{"n":` + string(rune('0'+i)) + `}`),
			HealthBaseline:     0.90 + float64(i)*0.02,
			CodeAcceptanceRate: 0.80 + float64(i)*0.05,
		}
		if err := store.Create(ctx, snap); err != nil {
			t.Fatalf("Create #%d failed: %v", i, err)
		}
	}

	// Create an unrelated snapshot to ensure isolation
	other := &KnowledgeSnapshot{
		ProjectID:    "other-proj",
		SnapshotData: json.RawMessage(`{"other":true}`),
	}
	if err := store.Create(ctx, other); err != nil {
		t.Fatalf("Create other failed: %v", err)
	}

	snapshots, err := store.ListByProject(ctx, "proj-b")
	if err != nil {
		t.Fatalf("ListByProject failed: %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 snapshots for proj-b, got %d", len(snapshots))
	}
	if snapshots[0].Version != 1 {
		t.Errorf("expected first version 1, got %d", snapshots[0].Version)
	}
	if snapshots[2].Version != 3 {
		t.Errorf("expected last version 3, got %d", snapshots[2].Version)
	}
}

func TestKnowledgeSnapshotRollback(t *testing.T) {
	ctx := context.Background()
	store := NewMemKnowledgeSnapshotStore()

	versions := make([]int, 5)
	for i := 0; i < 5; i++ {
		snap := &KnowledgeSnapshot{
			ProjectID:          "proj-c",
			SnapshotData:       json.RawMessage(`{"v":` + string(rune('0'+i)) + `}`),
			HealthBaseline:     0.90,
			CodeAcceptanceRate: 0.80,
		}
		if err := store.Create(ctx, snap); err != nil {
			t.Fatalf("Create #%d failed: %v", i, err)
		}
		versions[i] = snap.Version
	}

	// Rollback to version 2
	rollback, err := store.Rollback(ctx, "proj-c", 2)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if rollback == nil {
		t.Fatal("expected non-nil rollback snapshot")
	}
	if rollback.Version != 2 {
		t.Errorf("expected rollback to version 2, got %d", rollback.Version)
	}

	// Rollback to non-existent version returns nil
	missing, err := store.Rollback(ctx, "proj-c", 99)
	if err != nil {
		t.Fatalf("Rollback missing version failed: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for non-existent version, got %+v", missing)
	}

	// Rollback to version from different project returns nil
	noProj, err := store.Rollback(ctx, "nonexistent", 1)
	if err != nil {
		t.Fatalf("Rollback missing project failed: %v", err)
	}
	if noProj != nil {
		t.Errorf("expected nil for non-existent project, got %+v", noProj)
	}
}
