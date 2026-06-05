package adapter

import (
	"context"
	"database/sql"
	"fmt"

	"openforge/internal/agent/domain"
)

// PGCheckpointRepo persists checkpoints to PostgreSQL.
type PGCheckpointRepo struct {
	db *sql.DB
}

func NewPGCheckpointRepo(db *sql.DB) *PGCheckpointRepo {
	return &PGCheckpointRepo{db: db}
}

func (r *PGCheckpointRepo) Save(ctx context.Context, cp *domain.Checkpoint) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO checkpoint (pipeline_id, stage, seq, data, trigger, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cp.PipelineID, cp.Stage, cp.Seq, cp.Data, cp.Trigger, cp.CreatedAt)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

func (r *PGCheckpointRepo) LoadLatest(ctx context.Context, pipelineID string) (*domain.Checkpoint, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT pipeline_id, stage, seq, data, trigger, created_at
		 FROM checkpoint WHERE pipeline_id = $1
		 ORDER BY seq DESC LIMIT 1`, pipelineID)
	var cp domain.Checkpoint
	if err := row.Scan(&cp.PipelineID, &cp.Stage, &cp.Seq, &cp.Data, &cp.Trigger, &cp.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load latest checkpoint: %w", err)
	}
	return &cp, nil
}

func (r *PGCheckpointRepo) List(ctx context.Context, pipelineID string) ([]*domain.Checkpoint, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT pipeline_id, stage, seq, data, trigger, created_at
		 FROM checkpoint WHERE pipeline_id = $1
		 ORDER BY seq DESC`, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()
	var result []*domain.Checkpoint
	for rows.Next() {
		var cp domain.Checkpoint
		if err := rows.Scan(&cp.PipelineID, &cp.Stage, &cp.Seq, &cp.Data, &cp.Trigger, &cp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		result = append(result, &cp)
	}
	return result, rows.Err()
}

// compile-time interface check
var _ domain.CheckpointRepository = (*PGCheckpointRepo)(nil)
