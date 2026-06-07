// Package bench contains 4 categories of benchmarks for OpenForge per
// DESIGN §13.5: pipeline create throughput, CSP channel latency, embedding
// query p99, and sandbox allocate p99.
//
// Each bench degrades gracefully when its backing service is not available
// in the current environment: if the relevant env var is unset the bench
// is skipped (NOT failed) so the same suite works locally, on CI, and on
// developer laptops.
package bench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/google/uuid"

	"openforge/internal/pipeline/adapter"
	"openforge/internal/pipeline/domain"
)

// newBenchDB opens a *sql.DB using the BENCH_PG_DSN env var. Returns nil if
// the env var is unset (callers MUST treat nil as "skip").
func newBenchDB(b *testing.B) *sql.DB {
	b.Helper()
	dsn := os.Getenv("BENCH_PG_DSN")
	if dsn == "" {
		return nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		b.Fatalf("open BENCH_PG_DSN: %v", err)
	}
	// Tight pool so we measure pipeline create cost, not DB connection cost.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		b.Skipf("BENCH_PG_DSN unreachable: %v", err)
	}
	return db
}

// BenchmarkPipeline_CreateThroughput measures per-op latency of
// PGRepository.Create on the pipeline table. The cost of a single INSERT
// is the proxy for "how fast can a user create a pipeline" — the
// dominant hot path for the API layer.
//
// Run with: BENCH_PG_DSN=postgres://user:pass@host:5432/db go test -bench=.
func BenchmarkPipeline_CreateThroughput(b *testing.B) {
	db := newBenchDB(b)
	if db == nil {
		b.Skip("BENCH_PG_DSN not set — skipping pipeline bench")
	}
	defer db.Close()

	repo := adapter.NewPGRepository(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := repo.Create(ctx, &domain.Pipeline{
			ID:        uuid.NewString(),
			ProjectID: "p-bench",
			Title:     fmt.Sprintf("bench-%d", i),
			Level:     "L1",
			Status:    "pending",
			CreatedBy: "bench",
		})
		if err != nil {
			b.Fatalf("create %d: %v", i, err)
		}
	}
}
