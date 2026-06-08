package bench

import (
	"context"
	"sort"
	"testing"
	"time"

	"openforge/internal/adapter"
)

// newSandboxPool builds a fresh SandboxProvider for the bench. We use a
// tiny warm pool (size 1) so the bench exercises both the warm fast path
// and the cold-start path, which is the realistic production mix.
func newSandboxPool(b *testing.B) *adapter.SandboxProvider {
	b.Helper()
	return adapter.NewSandboxProvider(adapter.SandboxProviderConfig{
		WarmCount:   1,
		MaxTotal:    b.N + 10, // make sure we never hit the cap during the bench
		IdleTimeout: time.Hour,
		Image:       "openforge/sandbox-bench:latest",
	})
}

// BenchmarkSandbox_AllocateP99 measures per-op latency of
// SandboxProvider.Acquire (the warm-pool + cold-start path used by
// every agent tool execution). Tracks p99 in addition to ns/op — the
// metric we alert on in production.
//
// Always runnable: the provider is in-process and uses a noop runtime,
// so no Docker daemon is required.
func BenchmarkSandbox_AllocateP99(b *testing.B) {
	pool := newSandboxPool(b)
	defer pool.Drain()

	ctx := context.Background()
	latencies := make([]time.Duration, b.N)

	// Pre-warm so the first iteration doesn't pay cold-start cost.
	if sb, err := pool.Acquire(ctx); err == nil {
		pool.Release(sb)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		sb, err := pool.Acquire(ctx)
		latencies[i] = time.Since(start)
		if err != nil {
			b.Fatalf("acquire %d: %v", i, err)
		}
		if sb != nil {
			pool.Release(sb)
		}
	}
	b.StopTimer()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	pick := func(pct int) int {
		idx := b.N * pct / 100
		if idx >= b.N {
			idx = b.N - 1
		}
		return idx
	}
	// Note: warm-pool Acquire is sub-microsecond; the p50/95/99-us
	// metrics may round to 0 — read ns/op for the canonical figure.
	b.ReportMetric(float64(latencies[pick(50)].Microseconds()), "p50-us")
	b.ReportMetric(float64(latencies[pick(95)].Microseconds()), "p95-us")
	b.ReportMetric(float64(latencies[pick(99)].Microseconds()), "p99-us")
}
