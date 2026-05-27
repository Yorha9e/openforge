package domain

import (
	"testing"
	"time"
)

func TestSLOTracker_RecordsPipelineDuration(t *testing.T) {
	tracker := NewSLOTracker()
	tracker.RecordPipeline(100*time.Millisecond, true)
	tracker.RecordPipeline(200*time.Millisecond, false)

	snap := tracker.Snapshot()
	if snap.Total != 2 {
		t.Fatalf("expected total 2, got %d", snap.Total)
	}
}

func TestSLOTracker_ErrorBudgetCalculation(t *testing.T) {
	tracker := NewSLOTracker()
	tracker.RecordPipeline(100*time.Millisecond, true)
	tracker.RecordPipeline(200*time.Millisecond, false)

	snap := tracker.Snapshot()
	if snap.SuccessRate != 50.0 {
		t.Fatalf("expected success rate 50%%, got %f", snap.SuccessRate)
	}
}

func TestSLOTracker_SnapshotIsCopy(t *testing.T) {
	tracker := NewSLOTracker()
	tracker.RecordPipeline(100*time.Millisecond, true)

	snap1 := tracker.Snapshot()
	tracker.RecordPipeline(200*time.Millisecond, true)
	snap2 := tracker.Snapshot()

	if snap1.Total != 1 || snap2.Total != 2 {
		t.Fatalf("expected snapshots to be independent, got %d and %d", snap1.Total, snap2.Total)
	}
}
