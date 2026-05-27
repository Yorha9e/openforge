package adapter

import (
	"context"
	"database/sql"

	"openforge/internal/agent/domain"
)

// PGKnowledgeSnapshotStore implements domain.KnowledgeSnapshotStore backed by PostgreSQL.
type PGKnowledgeSnapshotStore struct {
	db *sql.DB
}

// NewPGKnowledgeSnapshotStore creates a new PGKnowledgeSnapshotStore.
func NewPGKnowledgeSnapshotStore(db *sql.DB) *PGKnowledgeSnapshotStore {
	return &PGKnowledgeSnapshotStore{db: db}
}

// Create inserts a new versioned knowledge snapshot, auto-incrementing the version
// for the target project.
func (s *PGKnowledgeSnapshotStore) Create(ctx context.Context, snap *domain.KnowledgeSnapshot) error {
	// Determine next version: max(version) + 1 for this project.
	var maxVer sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(version) FROM knowledge_snapshot WHERE project_id = $1
	`, snap.ProjectID).Scan(&maxVer)
	if err != nil {
		return err
	}
	nextVer := 1
	if maxVer.Valid {
		nextVer = int(maxVer.Int64) + 1
	}
	snap.Version = nextVer

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO knowledge_snapshot (project_id, version, snapshot_data, signature, health_baseline, code_acceptance_rate)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snap.ProjectID, snap.Version, snap.SnapshotData, snap.Signature,
		snap.HealthBaseline, snap.CodeAcceptanceRate)
	return err
}

// GetLatest returns the most recent snapshot for a project.
func (s *PGKnowledgeSnapshotStore) GetLatest(ctx context.Context, projectID string) (*domain.KnowledgeSnapshot, error) {
	snap := &domain.KnowledgeSnapshot{}
	var healthBase float64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, version, snapshot_data,
			COALESCE(signature, ''), health_baseline,
			COALESCE(code_acceptance_rate, 0), created_at
		FROM knowledge_snapshot
		WHERE project_id = $1
		ORDER BY version DESC
		LIMIT 1
	`, projectID).Scan(
		&snap.ID, &snap.ProjectID, &snap.Version, &snap.SnapshotData,
		&snap.Signature, &healthBase, &snap.CodeAcceptanceRate, &snap.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.HealthBaseline = healthBase
	return snap, nil
}

// ListByProject returns all snapshots for a project, newest first.
func (s *PGKnowledgeSnapshotStore) ListByProject(ctx context.Context, projectID string) ([]*domain.KnowledgeSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, version, snapshot_data,
			COALESCE(signature, ''), health_baseline,
			COALESCE(code_acceptance_rate, 0), created_at
		FROM knowledge_snapshot
		WHERE project_id = $1
		ORDER BY version DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*domain.KnowledgeSnapshot
	for rows.Next() {
		snap := &domain.KnowledgeSnapshot{}
		var healthBase float64
		if err := rows.Scan(
			&snap.ID, &snap.ProjectID, &snap.Version, &snap.SnapshotData,
			&snap.Signature, &healthBase, &snap.CodeAcceptanceRate, &snap.CreatedAt,
		); err != nil {
			return nil, err
		}
		snap.HealthBaseline = healthBase
		results = append(results, snap)
	}
	return results, rows.Err()
}

// Rollback retrieves a snapshot at a specific version.
func (s *PGKnowledgeSnapshotStore) Rollback(ctx context.Context, projectID string, version int) (*domain.KnowledgeSnapshot, error) {
	snap := &domain.KnowledgeSnapshot{}
	var healthBase float64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, version, snapshot_data,
			COALESCE(signature, ''), health_baseline,
			COALESCE(code_acceptance_rate, 0), created_at
		FROM knowledge_snapshot
		WHERE project_id = $1 AND version = $2
	`, projectID, version).Scan(
		&snap.ID, &snap.ProjectID, &snap.Version, &snap.SnapshotData,
		&snap.Signature, &healthBase, &snap.CodeAcceptanceRate, &snap.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.HealthBaseline = healthBase
	return snap, nil
}

// compile-time interface check
var _ domain.KnowledgeSnapshotStore = (*PGKnowledgeSnapshotStore)(nil)
