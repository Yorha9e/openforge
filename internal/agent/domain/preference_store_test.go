package domain

import (
	"context"
	"testing"
)

func TestMemPreferenceStore_UpsertAndList(t *testing.T) {
	store := NewMemPreferenceStore()
	ctx := context.Background()

	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 5.0, Source: "code_review"})
	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "error_handling", Value: "try-catch", Weight: 3.0, Source: "auto_detect"})

	list, err := store.ListByProject(ctx, "proj-A")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 preferences, got %d", len(list))
	}
}

func TestMemPreferenceStore_UpsertExisting(t *testing.T) {
	store := NewMemPreferenceStore()
	ctx := context.Background()

	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 3.0})
	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 7.0})

	list, _ := store.ListByProject(ctx, "proj-A")
	if len(list) != 1 {
		t.Fatalf("expected 1 preference after upsert, got %d", len(list))
	}
	if list[0].Weight != 7.0 {
		t.Errorf("expected weight 7.0, got %f", list[0].Weight)
	}
	if list[0].ConflictCount != 1 {
		t.Errorf("expected conflict_count 1, got %d", list[0].ConflictCount)
	}
}

func TestResolveConflict_FrequencyWins(t *testing.T) {
	records := []PreferenceRecord{
		{Key: "naming", Value: "camelCase", ConflictCount: 10, Weight: 3.0},
		{Key: "naming", Value: "snake_case", ConflictCount: 2, Weight: 9.0},
	}
	winner := ResolveConflict(records)
	if winner == nil || winner.Value != "camelCase" {
		t.Errorf("expected camelCase (higher frequency), got %v", winner)
	}
}

func TestResolveConflict_TimeBreaksTie(t *testing.T) {
	records := []PreferenceRecord{
		{Key: "naming", Value: "camelCase", ConflictCount: 5, LastActivated: "2026-05-20T00:00:00Z"},
		{Key: "naming", Value: "PascalCase", ConflictCount: 5, LastActivated: "2026-05-24T00:00:00Z"},
	}
	winner := ResolveConflict(records)
	if winner == nil || winner.Value != "PascalCase" {
		t.Errorf("expected PascalCase (more recent), got %v", winner)
	}
}

func TestMergeSimilarPreferences(t *testing.T) {
	prefs := []PreferenceRecord{
		{Key: "naming", Value: "camelCase"},
		{Key: "naming", Value: "camelCase"},  // duplicate
		{Key: "error_handling", Value: "try-catch"},
	}
	merged := MergeSimilarPreferences(prefs)
	if len(merged) != 2 {
		t.Errorf("expected 2 after merge, got %d", len(merged))
	}
}
