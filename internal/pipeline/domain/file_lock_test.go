package domain

import (
	"testing"
	"time"
)

func TestFileLock_IsExpired(t *testing.T) {
	future := FileLock{ExpiresAt: time.Now().Add(time.Hour)}
	if future.IsExpired() {
		t.Error("future lock should not be expired")
	}

	past := FileLock{ExpiresAt: time.Now().Add(-time.Hour)}
	if !past.IsExpired() {
		t.Error("past lock should be expired")
	}
}

func TestFileLock_Types(t *testing.T) {
	if LockWrite != "write" {
		t.Errorf("expected 'write', got %q", LockWrite)
	}
	if LockReadOnly != "read_only" {
		t.Errorf("expected 'read_only', got %q", LockReadOnly)
	}
}

func TestDetectCycles_EmptyGraph(t *testing.T) {
	cycles := DetectCycles(nil)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d", len(cycles))
	}

	cycles = DetectCycles(map[string]map[string]bool{})
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d", len(cycles))
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	// A -> B -> C (no cycle)
	adj := map[string]map[string]bool{
		"A": {"B": true},
		"B": {"C": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d: %+v", len(cycles), cycles)
	}
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	// A -> B -> A
	adj := map[string]map[string]bool{
		"A": {"B": true},
		"B": {"A": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %+v", len(cycles), cycles)
	}
	if len(cycles[0].PipelineIDs) < 2 {
		t.Fatalf("cycle should have at least 2 nodes, got %d", len(cycles[0].PipelineIDs))
	}
}

func TestDetectCycles_ComplexCycle(t *testing.T) {
	// A -> B -> C -> A
	adj := map[string]map[string]bool{
		"A": {"B": true},
		"B": {"C": true},
		"C": {"A": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %+v", len(cycles), cycles)
	}
	cycle := cycles[0]
	if len(cycle.PipelineIDs) != 3 {
		t.Fatalf("expected 3 nodes in cycle, got %d: %v", len(cycle.PipelineIDs), cycle.PipelineIDs)
	}
	// Verify all three nodes are present (order may vary)
	seen := make(map[string]bool)
	for _, id := range cycle.PipelineIDs {
		seen[id] = true
	}
	for _, node := range []string{"A", "B", "C"} {
		if !seen[node] {
			t.Errorf("cycle missing node %s: %v", node, cycle.PipelineIDs)
		}
	}
}

func TestDetectCycles_MultipleCycles(t *testing.T) {
	// Cycle 1: A -> B -> A
	// Cycle 2: C -> D -> E -> C
	// F -> G (no cycle)
	adj := map[string]map[string]bool{
		"A": {"B": true},
		"B": {"A": true},
		"C": {"D": true},
		"D": {"E": true},
		"E": {"C": true},
		"F": {"G": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 2 {
		t.Fatalf("expected 2 cycles, got %d: %+v", len(cycles), cycles)
	}
}

func TestDetectCycles_SelfLoop(t *testing.T) {
	// A -> A (self-loop is a trivial cycle)
	adj := map[string]map[string]bool{
		"A": {"A": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle (self-loop), got %d: %+v", len(cycles), cycles)
	}
	if len(cycles[0].PipelineIDs) < 1 {
		t.Fatal("self-loop cycle should have at least 1 node")
	}
}

func TestDetectCycles_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D (diamond, no cycle)
	adj := map[string]map[string]bool{
		"A": {"B": true, "C": true},
		"B": {"D": true},
		"C": {"D": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles in diamond graph, got %d: %+v", len(cycles), cycles)
	}
}

func TestDetectCycles_DisconnectedWithCycle(t *testing.T) {
	// A -> B (no cycle), C -> D -> E -> C (cycle)
	adj := map[string]map[string]bool{
		"A": {"B": true},
		"C": {"D": true},
		"D": {"E": true},
		"E": {"C": true},
	}
	cycles := DetectCycles(adj)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %+v", len(cycles), cycles)
	}
}
