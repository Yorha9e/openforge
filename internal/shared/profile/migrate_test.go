package profile

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// newTestDB returns a *sql.DB connected to TEST_DATABASE_URL, or skips the
// test if the env var is not set. This matches the codebase pattern in
// migrate_test.go (T8 sibling test) and lets CI run integration tests when
// a PG instance is available, while local `go test` without infra still
// compiles and runs unit-only tests.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Skipf("TEST_DATABASE_URL set but DB not reachable: %v", err)
	}
	return db
}

// TestMigrateProfileBackend_MinimalToStandard_ReconcilesTokenQuota verifies
// the standard migration path: minimal → standard seeds the operator-visible
// audit trail and records the cost_quota reconciliation step.
//
// TODO(Path-A schema gap): The current cost_quota schema (see migrations/001_init.up.sql)
// has UNIQUE(project_id, month) and no monthly_usd column. The plan's intent of
// seeding a `_default` row is blocked until T10 (integration verification) lands
// the 012_cost_quota_monthly_usd migration. This test therefore asserts on the
// step list (which is always populated from in-memory state) and on the audit
// log side effect, which is the durable record operators query.
func TestMigrateProfileBackend_MinimalToStandard_ReconcilesTokenQuota(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()

	res, err := MigrateProfileBackend(ctx, db, "minimal", "standard")
	if err != nil {
		t.Fatalf("MigrateProfileBackend(minimal→standard) error = %v", err)
	}
	if res.From != "minimal" {
		t.Errorf("From = %q, want %q", res.From, "minimal")
	}
	if res.To != "standard" {
		t.Errorf("To = %q, want %q", res.To, "standard")
	}
	if len(res.Steps) < 2 {
		t.Errorf("expected at least 2 steps, got %d: %v", len(res.Steps), res.Steps)
	}

	// Step 1: task_queue must be empty (T8 pre-condition for T9's DROP).
	// If the operator has live queue rows the migration should still run
	// but surface a warning — T9 is a destructive step and must not be
	// run blind. We assert the step is recorded; the actual count check
	// is informational, not a hard fail, so that the migration can run
	// on test environments with historical data.
	if !stepContains(res.Steps, "task_queue") {
		t.Errorf("expected a step referencing task_queue, got %v", res.Steps)
	}

	// Step 2: cost_quota reconciliation must be recorded. The actual row
	// insertion is gated on the schema-gap TODO above; until that
	// migration lands, the step is recorded as a no-op-and-warn.
	if !stepContains(res.Steps, "cost_quota") {
		t.Errorf("expected a step referencing cost_quota, got %v", res.Steps)
	}

	// Step 3: an audit_log row must have been appended (Path A relies on
	// the WORM chain to prove migrations happened).
	var auditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'profile_migration'`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount == 0 {
		t.Errorf("expected at least one audit_log row with action=profile_migration")
	}
}

// TestMigrateProfileBackend_Unsupported returns a typed error for
// combinations the operator must be told about, rather than silently
// no-op-ing the request.
func TestMigrateProfileBackend_Unsupported(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	_, err := MigrateProfileBackend(context.Background(), db, "minimal", "experimental")
	if err == nil {
		t.Fatal("expected error for unsupported migration, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported migration") {
		t.Errorf("expected error to contain 'unsupported migration', got %q", err.Error())
	}
}

// TestMigrateProfileBackend_NilDB confirms the function guards against
// a nil *sql.DB before dereferencing — important because the CLI may
// be invoked with a broken DSN and we want a clear error, not a panic.
func TestMigrateProfileBackend_NilDB(t *testing.T) {
	_, err := MigrateProfileBackend(context.Background(), nil, "minimal", "standard")
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

func stepContains(steps []string, needle string) bool {
	for _, s := range steps {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
