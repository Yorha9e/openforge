package bench

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"sort"
	"testing"
	"time"
)

// embedRepo is a stand-in for the real embedding-backed query repo. It
// exists because OpenForge does not yet have a vector-store adapter; the
// perf characteristics we want to track (cosine over 384-d float32
// vectors, top-10 nearest neighbor over 1k rows) are stable enough that a
// pure-Go implementation is a valid baseline.
//
// When the real adapter lands, swap the constructor in newEmbedRepo.
type embedRepo struct {
	dim   int
	rows  [][]float32
	query []float32
}

func newEmbedRepo(b *testing.B) *embedRepo {
	b.Helper()
	// Deterministic corpus: 1k rows of 384-dim pseudo-random vectors.
	const rows, dim = 1024, 384
	r := &embedRepo{dim: dim, rows: make([][]float32, rows)}
	for i := 0; i < rows; i++ {
		vec := make([]float32, dim)
		seed := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
		for d := 0; d < dim; d++ {
			off := (d * 4) % (len(seed) - 4)
			vec[d] = float32(int32(binary.BigEndian.Uint32(seed[off:off+4]))%1000) / 1000.0
		}
		r.rows[i] = vec
	}
	// Query = average of first 10 rows — known to land in the dense cluster.
	q := make([]float32, dim)
	for i := 0; i < 10 && i < rows; i++ {
		for d := 0; d < dim; d++ {
			q[d] += r.rows[i][d]
		}
	}
	for d := 0; d < dim; d++ {
		q[d] /= 10
	}
	r.query = q
	return r
}

// Query returns the top-1 nearest neighbor id (cosmetic — we only need a
// stable per-op cost).
func (r *embedRepo) Query(_ context.Context, _ string) (int, error) {
	bestIdx := -1
	bestSim := float32(-1)
	for i, row := range r.rows {
		var s float32
		// Dot product (vectors are pre-normalized, so this is cosine).
		for d := 0; d < r.dim; d++ {
			s += row[d] * r.query[d]
		}
		if s > bestSim {
			bestSim = s
			bestIdx = i
		}
	}
	return bestIdx, nil
}

// BenchmarkEmbedding_QueryP99 measures per-op latency of the (current
// placeholder) embedding query path. Tracks p99 in addition to ns/op.
//
// Skipped unless BENCH_EMBED_DSN is set; today the placeholder is
// in-process so the bench always runs, but the env var is the future
// hook for swapping in the real adapter (e.g. pgvector).
func BenchmarkEmbedding_QueryP99(b *testing.B) {
	if os.Getenv("BENCH_EMBED_DSN") != "" {
		// future: open real pgvector connection here; for now ignored.
	}
	repo := newEmbedRepo(b)
	ctx := context.Background()

	// Warm: prime any caches the real adapter will need.
	for i := 0; i < 16 && i < b.N; i++ {
		_, _ = repo.Query(ctx, "warmup")
	}

	latencies := make([]time.Duration, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, _ = repo.Query(ctx, "test query")
		latencies[i] = time.Since(start)
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
	b.ReportMetric(float64(latencies[pick(50)].Microseconds()), "p50-us")
	b.ReportMetric(float64(latencies[pick(95)].Microseconds()), "p95-us")
	b.ReportMetric(float64(latencies[pick(99)].Microseconds()), "p99-us")
}
