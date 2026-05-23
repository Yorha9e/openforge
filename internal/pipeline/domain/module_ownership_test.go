package domain

import "testing"

func TestOwnershipIndex_FindReviewers(t *testing.T) {
	ownerships := []ModuleOwnership{
		{ProjectID: "proj-A", Paths: []string{"frontend/"}, Reviewers: []string{"alice"}, FallbackReviewer: "bob"},
		{ProjectID: "proj-A", Paths: []string{"backend/"}, Reviewers: []string{"charlie"}, FallbackReviewer: "bob"},
	}
	idx := NewOwnershipIndex(ownerships)

	reviewers := idx.FindReviewers("proj-A", []string{"frontend/src/App.tsx"})
	if len(reviewers) != 1 || reviewers[0] != "alice" {
		t.Errorf("reviewers = %v, want [alice]", reviewers)
	}

	// Unknown path → fallback
	reviewers = idx.FindReviewers("proj-A", []string{"unknown/file.go"})
	if len(reviewers) < 1 {
		t.Error("fallback should return at least one reviewer")
	}
}

func TestOwnershipIndex_EmptyProject(t *testing.T) {
	idx := NewOwnershipIndex(nil)
	reviewers := idx.FindReviewers("nonexistent", []string{"foo.go"})
	if len(reviewers) != 0 {
		t.Errorf("expected 0 reviewers for unknown project, got %v", reviewers)
	}
}
