package adapter

import (
	"database/sql"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
)

// PGFileLockStore implements domain.FileLockStore using PostgreSQL.
type PGFileLockStore struct {
	db *sql.DB
}

// NewPGFileLockStore creates a new PGFileLockStore.
func NewPGFileLockStore(db *sql.DB) *PGFileLockStore {
	return &PGFileLockStore{db: db}
}

// Acquire inserts a file lock. Succeeds only when (project_id, file_path) is
// not already locked (ON CONFLICT DO NOTHING).
func (s *PGFileLockStore) Acquire(pipelineID, projectID, filePath string, lockType domain.LockType) error {
	res, err := s.db.Exec(`
		INSERT INTO file_lock
			(pipeline_id, project_id, file_path, lock_type, estimated_duration, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, file_path) DO NOTHING
	`, pipelineID, projectID, filePath, lockType, 300, time.Now().Add(10*time.Minute))
	if err != nil {
		return fmt.Errorf("acquire lock %s: %w", filePath, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrFileLockConflict
	}
	return nil
}

// Release deletes a file lock by project and file path.
func (s *PGFileLockStore) Release(projectID, filePath string) error {
	_, err := s.db.Exec(
		`DELETE FROM file_lock WHERE project_id=$1 AND file_path=$2`,
		projectID, filePath,
	)
	return err
}

// ListByProject returns all file locks for the given project.
func (s *PGFileLockStore) ListByProject(projectID string) ([]domain.FileLock, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(pipeline_id,''), project_id, file_path, lock_type,
		       estimated_duration, locked_at, expires_at
		FROM file_lock
		WHERE project_id = $1
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []domain.FileLock
	for rows.Next() {
		var l domain.FileLock
		if err := rows.Scan(
			&l.ID, &l.PipelineID, &l.ProjectID, &l.FilePath, &l.LockType,
			&l.EstimatedDuration, &l.LockedAt, &l.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan lock: %w", err)
		}
		locks = append(locks, l)
	}
	return locks, rows.Err()
}

// DetectDeadlock queries write-write lock conflicts and performs DFS-based
// cycle detection on the resulting dependency graph.
func (s *PGFileLockStore) DetectDeadlock(projectID string) ([]domain.GraphCycle, error) {
	rows, err := s.db.Query(`
		SELECT l1.pipeline_id, l1.file_path, l2.pipeline_id, l2.file_path
		FROM file_lock l1
		JOIN file_lock l2
		    ON l1.project_id = l2.project_id AND l1.file_path = l2.file_path
		WHERE l1.project_id = $1
		  AND l1.pipeline_id != l2.pipeline_id
		  AND l1.lock_type = 'write'
		  AND l2.lock_type = 'write'
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query conflicts: %w", err)
	}
	defer rows.Close()

	// Build adjacency list: pipeline1 → pipeline2 (p1 waits on p2 because
	// both hold a write lock on the same file).
	adj := make(map[string]map[string]bool)
	for rows.Next() {
		var p1ID, p1File, p2ID, p2File string
		if err := rows.Scan(&p1ID, &p1File, &p2ID, &p2File); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		if adj[p1ID] == nil {
			adj[p1ID] = make(map[string]bool)
		}
		adj[p1ID][p2ID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return domain.DetectCycles(adj), nil
}
