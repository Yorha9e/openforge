package adapter

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"openforge/internal/pipeline/domain"
)

// ownershipExecResult is the subset of sql.Result that PGOwnershipRepository
// needs. Decoupling from sql.Result makes the repository testable with
// hand-rolled fakes (see pg_ownership_repository_test.go).
type ownershipExecResult interface {
	RowsAffected() (int64, error)
}

// ownershipDB is the minimal SQL surface used by PGOwnershipRepository.
// *sql.DB satisfies this interface implicitly (via pgOwnershipDBAdapter).
type ownershipDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (ownershipExecResult, error)
	QueryOwnershipByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error)
}

// PGOwnershipRepository implements service.OwnershipRepository backed by
// PostgreSQL. It owns a narrow ownershipDB interface so the production
// path uses *sql.DB and tests can use a fake.
type PGOwnershipRepository struct {
	db ownershipDB
}

// NewPGOwnershipRepository creates a repository backed by the given
// PostgreSQL connection. The wrapper adapts *sql.DB's *sql.Result to
// the narrow ownershipExecResult interface the repository uses.
func NewPGOwnershipRepository(db *sql.DB) *PGOwnershipRepository {
	return &PGOwnershipRepository{db: &pgOwnershipDBAdapter{db: db}}
}

// newOwnershipRepoFromDB is the test-only constructor that takes a
// hand-rolled ownershipDB implementation. Production code uses
// NewPGOwnershipRepository.
func newOwnershipRepoFromDB(db ownershipDB) *PGOwnershipRepository {
	return &PGOwnershipRepository{db: db}
}

// pgOwnershipDBAdapter wraps *sql.DB to satisfy ownershipDB. *sql.DB
// returns *sql.Result which exposes RowsAffected() and LastInsertId();
// we only need RowsAffected.
type pgOwnershipDBAdapter struct {
	db *sql.DB
}

func (a *pgOwnershipDBAdapter) ExecContext(ctx context.Context, query string, args ...any) (ownershipExecResult, error) {
	res, err := a.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &sqlExecResult{res: res}, nil
}

func (a *pgOwnershipDBAdapter) QueryOwnershipByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT project_id, module_name, paths, team_name, reviewers, fallback_reviewer
		   FROM module_ownership
		  WHERE project_id = $1
		  ORDER BY module_name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ModuleOwnership
	for rows.Next() {
		var mo domain.ModuleOwnership
		if err := rows.Scan(&mo.ProjectID, &mo.ModuleName, pq.Array(&mo.Paths), &mo.TeamName, pq.Array(&mo.Reviewers), &mo.FallbackReviewer); err != nil {
			return nil, fmt.Errorf("scan module_ownership row: %w", err)
		}
		out = append(out, mo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module_ownership rows: %w", err)
	}
	return out, nil
}

// sqlExecResult adapts *sql.Result to ownershipExecResult.
type sqlExecResult struct {
	res sql.Result
}

func (r *sqlExecResult) RowsAffected() (int64, error) { return r.res.RowsAffected() }

// ListByProject returns all module ownership records for a given project.
func (r *PGOwnershipRepository) ListByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error) {
	return r.db.QueryOwnershipByProject(ctx, projectID)
}

// Upsert inserts or updates a module ownership record. The unique key is
// (project_id, module_name); on conflict the existing row is overwritten
// with the new paths/team/reviewers/fallback.
func (r *PGOwnershipRepository) Upsert(ctx context.Context, mo domain.ModuleOwnership) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO module_ownership
			(project_id, module_name, paths, team_name, reviewers, fallback_reviewer)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, module_name) DO UPDATE SET
			paths = EXCLUDED.paths,
			team_name = EXCLUDED.team_name,
			reviewers = EXCLUDED.reviewers,
			fallback_reviewer = EXCLUDED.fallback_reviewer
	`, mo.ProjectID, mo.ModuleName, pq.Array(mo.Paths), mo.TeamName, pq.Array(mo.Reviewers), mo.FallbackReviewer)
	if err != nil {
		return fmt.Errorf("upsert module_ownership: %w", err)
	}
	return nil
}
