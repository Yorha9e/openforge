package profile

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

// MigrationRunner executes .up.sql files from a directory in lexicographic order,
// tracking executed migrations in schema_migrations for idempotency.
type MigrationRunner struct {
	db           *sql.DB
	dir          string
	migrationDSN string // empty when constructed via NewMigrationRunner(db, dir)
}

func NewMigrationRunner(db *sql.DB, dir string) *MigrationRunner {
	return &MigrationRunner{db: db, dir: dir}
}

// NewMigrationRunnerFromDSN opens a connection using the migration DSN
// (X3 T3 #19) and returns a runner bound to that connection. The returned
// runner owns the underlying *sql.DB; callers should call Close() to release it.
func NewMigrationRunnerFromDSN(migrationDSN, dir string) (*MigrationRunner, error) {
	db, err := sql.Open("postgres", migrationDSN)
	if err != nil {
		return nil, fmt.Errorf("migrate: open migration dsn: %w", err)
	}
	return &MigrationRunner{db: db, dir: dir, migrationDSN: migrationDSN}, nil
}

// Close releases the database connection held by the runner. Only safe
// to call when the runner was created via NewMigrationRunnerFromDSN.
// For runners created with NewMigrationRunner(db, dir) the caller owns
// the *sql.DB and must close it themselves.
func (r *MigrationRunner) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

// MigrationDSN reports the DSN the runner is using. When constructed via
// NewMigrationRunner(db, dir) the DSN is empty (the db was injected).
func (r *MigrationRunner) DSN() string {
	if r.migrationDSN != "" {
		return r.migrationDSN
	}
	return ""
}

// Run executes all pending .up.sql files, each in its own transaction.
//
// Before running any DDL, Run performs a failover gate check: it asks the
// target PostgreSQL instance whether it is in recovery mode (a hot-standby
// replica serving read traffic during a failover, or an old primary that has
// been demoted). DDL on a recovery-mode server is impossible, but more
// importantly we want migrations to run on the *intended* primary, not a
// read-only replica that may have just been promoted mid-migration.
//
// On gate refusal, Run returns a non-nil error before executing anything.
func (r *MigrationRunner) Run(ctx context.Context) error {
	// 1. Failover gate: refuse to run DDL against a server in recovery.
	var inRecovery bool
	if err := r.db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return fmt.Errorf("migration gate: check pg state: %w", err)
	}
	if inRecovery {
		return fmt.Errorf("migration gate: PG in recovery mode, refusing to run DDL")
	}
	// 2. Existing migration execution logic.
	if err := r.ensureTrackingTable(ctx); err != nil {
		return fmt.Errorf("migrate: tracking table: %w", err)
	}

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("migrate: read dir %s: %w", r.dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		executed, err := r.isExecuted(ctx, f)
		if err != nil {
			return fmt.Errorf("migrate: check %s: %w", f, err)
		}
		if executed {
			continue
		}

		content, err := os.ReadFile(filepath.Join(r.dir, f))
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", f, err)
		}

		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", f, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate: exec %s: %w", f, err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, f); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate: track %s: %w", f, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", f, err)
		}
	}
	return nil
}

func (r *MigrationRunner) ensureTrackingTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename    VARCHAR(255) PRIMARY KEY,
			executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (r *MigrationRunner) isExecuted(ctx context.Context, filename string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, filename).Scan(&exists)
	return exists, err
}
