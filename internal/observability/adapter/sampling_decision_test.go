package adapter

import (
	"fmt"
	"log/slog"
	"testing"
)

func TestShouldSample_WarnAlwaysKept(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		msg := "warn-message"
		if d := ShouldSample(slog.LevelWarn, msg); d != Keep {
			t.Fatalf("WARN level must always KEEP, got %v on iter %d", d, i)
		}
	}
}

func TestShouldSample_InfoSamples10Percent(t *testing.T) {
	const iterations = 1000
	const tolerance = 30
	kept := 0
	for i := 0; i < iterations; i++ {
		// Vary the message each iteration so the hash distribution is
		// exercised across the full space.
		msg := fmt.Sprintf("info-message-%d", i)
		if d := ShouldSample(slog.LevelInfo, msg); d == Keep {
			kept++
		}
	}
	expected := iterations / 10
	if kept < expected-tolerance || kept > expected+tolerance {
		t.Fatalf("expected ~%d KEEP (10%% of %d, tolerance %d), got %d",
			expected, iterations, tolerance, kept)
	}
}

func TestShouldSample_DebugAlwaysDropped(t *testing.T) {
	for i := 0; i < 100; i++ {
		if d := ShouldSample(slog.LevelDebug, "dbg"); d != Drop {
			t.Fatalf("DEBUG level must always DROP, got %v", d)
		}
	}
}
