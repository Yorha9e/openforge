package profile

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MigrationRunner executes .up.sql files from a directory in lexicographic order,
// tracking executed migrations in schema_migrations for idempotency.
type MigrationRunner struct {
	db  *sql.DB
	dir string
}

func NewMigrationRunner(db *sql.DB, dir string) *MigrationRunner {
	return &MigrationRunner{db: db, dir: dir}
}

// Run executes all pending .up.sql files, each in its own transaction.
func (r *MigrationRunner) Run(ctx context.Context) error {
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
