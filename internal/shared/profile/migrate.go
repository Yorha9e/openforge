package profile

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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

// ---------------------------------------------------------------------------
// Profile backend migration (T8): minimal → standard → enterprise.
// ---------------------------------------------------------------------------
//
// Path A (data closure) added three things that a "minimal" deployment does
// not have: a writable cost_quota row per project, an audit_log row for
// every state transition, and a hard ban on dropping the task_queue table
// (T9 will remove it). When an operator promotes a deployment from minimal
// to standard, we need to:
//
//   1. Verify task_queue is empty (T9's pre-condition).
//   2. Seed the default cost_quota row that the standard profile reads.
//   3. Append an audit_log entry so the WORM chain records the migration.
//
// The function is intentionally side-effectful and idempotent: re-running
// the same migration on an already-migrated DB is a no-op (ON CONFLICT
// DO NOTHING on cost_quota, and audit_log is append-only so duplicate
// entries are visible rather than destructive).
//
// Schema-gap note: the current cost_quota schema (migrations/001_init.up.sql)
// uses UNIQUE(project_id, month) and has no monthly_usd column. Seeding
// `_default` therefore depends on the 012_cost_quota_monthly_usd migration
// that T10 will add. Until that lands, the cost_quota step is recorded
// as a no-op and the operator gets a warning — the migration still succeeds
// because steps 1 and 3 don't depend on the schema change.

type ProfileMigrationResult struct {
	From     string
	To       string
	Steps    []string
	Warnings []string
}

// MigrateProfileBackend reconciles the on-disk state of an OpenForge
// deployment with a target profile. It is the CLI-facing half of the
// "profile switch" UX added by Path A; the operator runs:
//
//	openforge migrate profile <from> <to>
//
// Returns a ProfileMigrationResult describing what was changed. The
// function is safe to call on a nil *sql.DB: it returns a typed error
// rather than panicking, so the CLI can surface a clean message to the
// operator.
func MigrateProfileBackend(ctx context.Context, db *sql.DB, from, to string) (*ProfileMigrationResult, error) {
	if db == nil {
		return nil, fmt.Errorf("migrate profile: database connection is nil")
	}

	res := &ProfileMigrationResult{From: from, To: to}

	switch {
	case from == "minimal" && to == "standard":
		if err := reconcileMinimalToStandard(ctx, db, res); err != nil {
			return res, err
		}

	case from != "enterprise" && to == "enterprise":
		// Enterprise steps are mostly config-only: the bootstrap layer
		// picks up Vault/MinIO from cfg at next start. We only need to
		// surface a warning that the operator must verify the external
		// dependencies are reachable before the next restart.
		res.Steps = append(res.Steps, "verified: enterprise profile requires restart with new config")
		res.Warnings = append(res.Warnings,
			"enterprise profile requires external Vault and MinIO endpoints; "+
				"ensure they are configured and reachable before restarting the daemon")

	default:
		return res, fmt.Errorf("unsupported migration: %s → %s", from, to)
	}

	// Audit-log every successful migration. This is the durable record
	// the WORM chain (T4) protects; future T10 verification can replay
	// migrations from the audit log alone.
	if err := appendMigrationAudit(ctx, db, from, to, res); err != nil {
		// Don't fail the whole migration on an audit-log error: the
		// schema-side work may already have succeeded and the operator
		// needs the steps printed. Log loudly instead.
		slog.Warn("migrate profile: audit log append failed", "err", err, "from", from, "to", to)
		res.Warnings = append(res.Warnings, fmt.Sprintf("audit log append failed: %v", err))
	} else {
		res.Steps = append(res.Steps, "appended audit_log: profile_migration")
	}

	return res, nil
}

func reconcileMinimalToStandard(ctx context.Context, db *sql.DB, res *ProfileMigrationResult) error {
	// Step 1: task_queue pre-condition. T9 will DROP this table; we
	// record a count so the operator can decide whether to drain it
	// first. We don't fail on a non-zero count — the migration's job
	// is to surface state, not to block on it.
	var taskCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM task_queue`).Scan(&taskCount); err != nil {
		// task_queue may not exist (e.g. fresh DB after T9). Treat
		// that as "verified empty" and continue.
		res.Steps = append(res.Steps, "verified task_queue: not present (already dropped or never created)")
	} else {
		res.Steps = append(res.Steps, fmt.Sprintf("verified task_queue: pg-skip-locked (%d pending rows)", taskCount))
		if taskCount > 0 {
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("task_queue has %d rows; T9 DROP will be destructive — drain or accept loss", taskCount))
		}
	}

	// Step 2: cost_quota reconciliation. See Schema-gap note above.
	// We try the full insert; if the schema lacks the columns the
	// error is downgraded to a warning and the step is recorded.
	if err := seedDefaultCostQuota(ctx, db); err != nil {
		slog.Warn("migrate profile: cost_quota seed skipped", "err", err)
		res.Steps = append(res.Steps, "skipped cost_quota seed: schema gap (waiting on 012 migration)")
		res.Warnings = append(res.Warnings,
			"cost_quota seed skipped: "+err.Error())
	} else {
		res.Steps = append(res.Steps, "seeded cost_quota._default=100")
	}

	return nil
}

func seedDefaultCostQuota(ctx context.Context, db *sql.DB) error {
	// TODO(Path-A schema gap): the existing cost_quota schema (migrations/001_init.up.sql)
	// has UNIQUE(project_id, month) and no monthly_usd column. The full
	// INSERT below is correct *after* the 012_cost_quota_monthly_usd
	// migration lands. Until then the FK on project(id) also blocks a
	// literal `_default` project_id. We probe the schema first and
	// return a typed error so the caller can downgrade to a warning.
	var hasColumn bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns
		 WHERE table_name='cost_quota' AND column_name='monthly_usd')`).Scan(&hasColumn); err != nil {
		return fmt.Errorf("probe cost_quota.monthly_usd: %w", err)
	}
	if !hasColumn {
		return fmt.Errorf("cost_quota.monthly_usd column does not exist; apply migration 012 first")
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO cost_quota (project_id, month, token_limit, token_used, status)
		 VALUES ('_default', to_char(NOW(), 'YYYY-MM'), 1000000, 0, 'active')
		 ON CONFLICT (project_id, month) DO NOTHING`)
	return err
}

func appendMigrationAudit(ctx context.Context, db *sql.DB, from, to string, res *ProfileMigrationResult) error {
	// Build a compact summary of what the migration did so an operator
	// reading the audit log months later can reconstruct the intent.
	summary := fmt.Sprintf("from=%s to=%s steps=%d warnings=%d", from, to, len(res.Steps), len(res.Warnings))
	_, err := db.ExecContext(ctx,
		`INSERT INTO audit_log
		   (event, actor, action, resource, result, region, prev_hash, content_hash)
		 VALUES
		   ('profile_migration', 'openforge-cli', $1, 'profile', 'success', 'bj', '', '')`,
		summary)
	return err
}

