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

// TestSLOTracker_P95_ReturnsSorted95thPercentile feeds the tracker 1..100ms
// in shuffled order. After sorting ascending, the 95th percentile index is
// int(0.95 * 100) = 95, which is the 96th element (96ms).
func TestSLOTracker_P95_ReturnsSorted95thPercentile(t *testing.T) {
	tracker := NewSLOTracker()
	// Build 1..100ms and shuffle so the test proves we actually sort.
	const N = 100
	durations := make([]time.Duration, N)
	for i := 0; i < N; i++ {
		durations[i] = time.Duration(i+1) * time.Millisecond
	}
	// Deterministic shuffle: reverse order.
	for i, j := 0, N-1; i < j; i, j = i+1, j-1 {
		durations[i], durations[j] = durations[j], durations[i]
	}
	for _, d := range durations {
		tracker.RecordPipeline(d, true)
	}

	got := tracker.P95()
	// 0.95 * 100 = 95, sorted[95] = 96ms.
	if got != 96 {
		t.Fatalf("P95() = %d, want 96", got)
	}
}

// TestSLOTracker_P95_HandlesUnorderedInput validates that a typical
// production stream (inserted in non-sorted order) yields a correct P95.
func TestSLOTracker_P95_HandlesUnorderedInput(t *testing.T) {
	tracker := NewSLOTracker()
	// 20 samples: 1ms, 2ms, ..., 20ms (in reverse order).
	for i := 20; i >= 1; i-- {
		tracker.RecordPipeline(time.Duration(i)*time.Millisecond, true)
	}
	// 0.95 * 20 = 19; sorted[19] = 20ms.
	got := tracker.P95()
	if got != 20 {
		t.Fatalf("P95() = %d, want 20", got)
	}
}

// TestSLOTracker_P95_EmptyReturnsZero guards the empty case.
func TestSLOTracker_P95_EmptyReturnsZero(t *testing.T) {
	tracker := NewSLOTracker()
	if got := tracker.P95(); got != 0 {
		t.Fatalf("P95() on empty tracker = %d, want 0", got)
	}
}
