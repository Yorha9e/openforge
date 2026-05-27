package adapter

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"openforge/internal/agent/domain"
)

// PGRetrospectiveStore implements domain.RetrospectiveStore backed by PostgreSQL.
type PGRetrospectiveStore struct {
	db *sql.DB
}

// NewPGRetrospectiveStore creates a new PGRetrospectiveStore.
func NewPGRetrospectiveStore(db *sql.DB) *PGRetrospectiveStore {
	return &PGRetrospectiveStore{db: db}
}

// Create persists a new pipeline retrospective.
func (s *PGRetrospectiveStore) Create(ctx context.Context, r *domain.PipelineRetrospective) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pipeline_retrospective
			(pipeline_id, project_id, duration_seconds, chat_rounds, total_tokens,
			 rejection_count, backtrack_count, lessons_learned, improvement_actions, knowledge_updates)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.PipelineID, r.ProjectID, r.DurationSeconds, r.ChatRounds, r.TotalTokens,
		r.RejectionCount, r.BacktrackCount,
		pq.Array(r.LessonsLearned), pq.Array(r.ImprovementActions), pq.Array(r.KnowledgeUpdates))
	return err
}

// ListByProject returns recent retrospectives for a project, limited by count.
func (s *PGRetrospectiveStore) ListByProject(ctx context.Context, projectID string, limit int) ([]domain.PipelineRetrospective, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pipeline_id, project_id, COALESCE(duration_seconds, 0),
			COALESCE(chat_rounds, 0), COALESCE(total_tokens, 0),
			COALESCE(rejection_count, 0), COALESCE(backtrack_count, 0),
			COALESCE(lessons_learned, '{}'), COALESCE(improvement_actions, '{}'),
			COALESCE(knowledge_updates, '{}'), created_at
		FROM pipeline_retrospective
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.PipelineRetrospective
	for rows.Next() {
		var r domain.PipelineRetrospective
		if err := rows.Scan(
			&r.PipelineID, &r.ProjectID, &r.DurationSeconds,
			&r.ChatRounds, &r.TotalTokens,
			&r.RejectionCount, &r.BacktrackCount,
			pq.Array(&r.LessonsLearned), pq.Array(&r.ImprovementActions),
			pq.Array(&r.KnowledgeUpdates), &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetByPipeline retrieves a single retrospective by pipeline ID.
func (s *PGRetrospectiveStore) GetByPipeline(ctx context.Context, pipelineID string) (*domain.PipelineRetrospective, error) {
	r := &domain.PipelineRetrospective{}
	err := s.db.QueryRowContext(ctx, `
		SELECT pipeline_id, project_id, COALESCE(duration_seconds, 0),
			COALESCE(chat_rounds, 0), COALESCE(total_tokens, 0),
			COALESCE(rejection_count, 0), COALESCE(backtrack_count, 0),
			COALESCE(lessons_learned, '{}'), COALESCE(improvement_actions, '{}'),
			COALESCE(knowledge_updates, '{}'), created_at
		FROM pipeline_retrospective
		WHERE pipeline_id = $1
	`, pipelineID).Scan(
		&r.PipelineID, &r.ProjectID, &r.DurationSeconds,
		&r.ChatRounds, &r.TotalTokens,
		&r.RejectionCount, &r.BacktrackCount,
		pq.Array(&r.LessonsLearned), pq.Array(&r.ImprovementActions),
		pq.Array(&r.KnowledgeUpdates), &r.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// compile-time interface check
var _ domain.RetrospectiveStore = (*PGRetrospectiveStore)(nil)
