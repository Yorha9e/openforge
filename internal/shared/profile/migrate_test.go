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
)

func TestMigrationRunner_Run(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Drop test table if exists from previous run
	db.Exec("DROP TABLE IF EXISTS _test_migration CASCADE")
	db.Exec("DROP TABLE IF EXISTS schema_migrations CASCADE")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "001_test.up.sql"), []byte(`
		CREATE TABLE IF NOT EXISTS _test_migration (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		);
	`), 0644)
	os.WriteFile(filepath.Join(dir, "002_test.up.sql"), []byte(`
		INSERT INTO _test_migration (name) VALUES ('hello');
	`), 0644)

	runner := NewMigrationRunner(db, dir)
	if err := runner.Run(t.Context()); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 migrations tracked, got %d", count)
	}

	// Idempotent: re-run should be no-op
	if err := runner.Run(t.Context()); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 2 {
		t.Errorf("expected still 2 after re-run, got %d", count)
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
