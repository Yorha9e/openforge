package uuid

import (
	"regexp"
	"testing"
	"time"
)

// TestNew_Format verifies that New() returns a valid RFC 4122 v7 UUID
// (8-4-4-4-12 hex, version=7, variant=10xx).
func TestNew_Format(t *testing.T) {
	u := New()
	if len(u) != 36 {
		t.Fatalf("expected 36-char UUID, got %d: %q", len(u), u)
	}
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(u) {
		t.Fatalf("UUID %q does not match v7 regex", u)
	}
}

// TestNew_Monotonic verifies that two consecutive calls produce UUIDs whose
// first 48 bits (timestamp_ms) are monotonically non-decreasing — the
// defining property of v7 that gives B-tree friendly ordering.
func TestNew_Monotonic(t *testing.T) {
	// Generate with a small sleep so that the clock has a chance to advance
	// even on fast machines.
	first := New()
	time.Sleep(2 * time.Millisecond)
	second := New()

	firstTS := first[:8] + first[9:13]
	secondTS := second[:8] + second[9:13]

	// Compare the 12-hex timestamp prefix lexicographically as a stand-in
	// for numeric comparison. Since each char is one hex digit, lex
	// ordering matches numeric ordering for same-length strings.
	if firstTS > secondTS {
		t.Fatalf("v7 timestamps not monotonic: first=%s second=%s", firstTS, secondTS)
	}
}

// TestNew_Unique verifies that 1000 calls produce 1000 distinct IDs.
func TestNew_Unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		u := New()
		if seen[u] {
			t.Fatalf("duplicate UUID on iteration %d: %s", i, u)
		}
		seen[u] = true
	}
}
