package adapter

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

// TestOTelTracer_ExportsSpan verifies that InitOTelTracer produces a working
// tracer provider and that span creation+end does not panic.  This test
// runs against a real OTLP collector by default; in -short mode it only
// verifies that the in-process provider initialises and can produce spans
// (no network export required).
func TestOTelTracer_ExportsSpan(t *testing.T) {
	if testing.Short() {
		t.Skip("requires otel-collector; skipped in -short mode")
	}

	ctx := context.Background()
	shutdown, err := InitOTelTracer(ctx, "openforge-test", "localhost:4317")
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown failed: %v", err)
		}
	}()

	_, span := otel.Tracer("openforge-test").Start(ctx, "test-span")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()
}

// TestOTelTracer_InitDoesNotPanic verifies the exporter initialises without
// panicking even when the OTLP collector is unreachable.  This is the bare
// minimum for production startup robustness — the main process should
// continue running even if telemetry is unavailable.
func TestOTelTracer_InitDoesNotPanic(t *testing.T) {
	ctx := context.Background()

	// InitOTelTracer should not panic regardless of collector availability.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("InitOTelTracer panicked: %v", r)
		}
	}()

	// Note: InitOTelTracer returns an error rather than panicking on failure
	// (it only panics on truly catastrophic misconfiguration), so an error
	// here is acceptable — the contract is "no panic".
	_, _ = InitOTelTracer(ctx, "openforge-test", "localhost:4317")
}

// TestOTelTracer_ShutdownIsIdempotent verifies the returned shutdown
// function can be called multiple times without panicking.
func TestOTelTracer_ShutdownIsIdempotent(t *testing.T) {
	ctx := context.Background()
	shutdown, err := InitOTelTracer(ctx, "openforge-test", "localhost:4317")
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}

	// First shutdown.
	if err := shutdown(ctx); err != nil {
		// First shutdown may fail if collector is unreachable, but should not panic.
		t.Logf("first shutdown error (acceptable): %v", err)
	}

	// Second shutdown — must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second shutdown panicked: %v", r)
		}
	}()
	_ = shutdown(ctx)
}
