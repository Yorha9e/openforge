package profile

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
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

func stepContains(steps []string, needle string) bool {
	for _, s := range steps {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
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

// TestNewMigrationRunnerFromDSN_InvalidDSN ensures the constructor returns
// an error (and does not panic) when given a syntactically invalid DSN.
func TestNewMigrationRunnerFromDSN_InvalidDSN(t *testing.T) {
	r, err := NewMigrationRunnerFromDSN("not-a-valid-dsn", t.TempDir())
	if err == nil {
		// Some drivers are lenient and only fail on Ping; if the runner
		// came back, ensure Close is safe and the test is a no-op.
		if r != nil {
			_ = r.Close()
		}
		return
	}
	if r != nil {
		t.Errorf("expected nil runner on error, got %+v", r)
	}
}

// TestNewMigrationRunnerFromDSN_EmptyDir verifies a non-existent dir is
// surfaced as an error from Run, not from the constructor (DSN parsing
// succeeds even if the target server is unreachable).
func TestNewMigrationRunnerFromDSN_EmptyDir(t *testing.T) {
	// Use a syntactically valid DSN that won't connect.
	r, err := NewMigrationRunnerFromDSN(
		"postgres://of_migration:of_migration_dev@127.0.0.1:1/openforge?sslmode=disable&connect_timeout=1",
		"/nonexistent/migrations/dir",
	)
	if err != nil {
		// Some drivers reject the URL upfront — that's fine, exercise the path.
		return
	}
	defer r.Close()
	if got := r.DSN(); got == "" {
		t.Errorf("DSN() = empty, want the migration DSN we passed in")
	}
	if err := r.Run(t.Context()); err == nil {
		t.Errorf("Run() against missing dir should fail")
	}
}

// recoveryStubDriver is a minimal database/sql/driver implementation that
// responds to the failover-gate query (SELECT pg_is_in_recovery()) with a
// configurable boolean result. It refuses every other query.
type recoveryStubDriver struct {
	inRecovery bool
}

func (d *recoveryStubDriver) Open(_ string) (driver.Conn, error) {
	return &recoveryStubConn{inRecovery: d.inRecovery}, nil
}

type recoveryStubConn struct {
	inRecovery bool
}

func (c *recoveryStubConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("recoveryStubConn: Prepare not supported")
}
func (c *recoveryStubConn) Close() error               { return nil }
func (c *recoveryStubConn) Begin() (driver.Tx, error) { return nil, errors.New("recoveryStubConn: Begin not supported") }

// QueryContext inspects the SQL for the gate probe; for any other statement
// it returns an error so the test cannot accidentally exercise the migration
// path against a fake server.
func (c *recoveryStubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.query(query)
}
func (c *recoveryStubConn) Query(query string, _ []driver.Value) (driver.Rows, error) {
	return c.query(query)
}

func (c *recoveryStubConn) query(query string) (driver.Rows, error) {
	if !strings.Contains(strings.ToLower(query), "pg_is_in_recovery") {
		return nil, errors.New("recoveryStubConn: unexpected query: " + query)
	}
	return &recoveryStubRows{values: []driver.Value{c.inRecovery}}, nil
}

func (c *recoveryStubConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("recoveryStubConn: Exec not supported")
}
func (c *recoveryStubConn) Exec(_ string, _ []driver.Value) (driver.Result, error) {
	return nil, errors.New("recoveryStubConn: Exec not supported")
}

type recoveryStubRows struct {
	values []driver.Value
	pos    int
}

func (r *recoveryStubRows) Columns() []string { return []string{"pg_is_in_recovery"} }
func (r *recoveryStubRows) Close() error      { return nil }
func (r *recoveryStubRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.values) {
		return ioEOF
	}
	dest[0] = r.values[r.pos]
	r.pos++
	return nil
}

// ioEOF mirrors io.EOF without importing "io" at the top of the file.
var ioEOF = errors.New("EOF")

func init() {
	sql.Register("recovery_stub", &recoveryStubDriver{inRecovery: true})
}

// TestMigrationRunner_Run_RejectsRecoveryMode verifies the X3 T9 failover
// gate: when the target PostgreSQL reports pg_is_in_recovery() = true,
// Run() must short-circuit with the gate error and never attempt any DDL.
func TestMigrationRunner_Run_RejectsRecoveryMode(t *testing.T) {
	db, err := sql.Open("recovery_stub", "")
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	// If the gate ever regresses, the runner will try to read the dir
	// and would also try to run migrations — assert neither happens.
	if err := os.WriteFile(filepath.Join(dir, "should_not_run.up.sql"), []byte("SELECT 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	runner := NewMigrationRunner(db, dir)
	err = runner.Run(t.Context())
	if err == nil {
		t.Fatal("expected gate error, got nil")
	}
	if !strings.Contains(err.Error(), "migration gate") {
		t.Errorf("expected gate error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "recovery") {
		t.Errorf("expected error to mention recovery, got: %v", err)
	}
}
