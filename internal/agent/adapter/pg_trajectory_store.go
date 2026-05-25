package adapter

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"openforge/internal/agent/domain"
)

type PGTrajectoryStore struct {
	db *sql.DB
}

func NewPGTrajectoryStore(db *sql.DB) *PGTrajectoryStore {
	return &PGTrajectoryStore{db: db}
}

func (s *PGTrajectoryStore) Record(ctx context.Context, t domain.TrajectoryRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trajectory (project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, requirement_summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, t.ProjectID, t.PipelineID, pq.Array(t.StageSequence), t.TotalChatRounds,
		t.TotalTokens, t.BacktrackCount, t.RejectionCount,
		pq.Array(t.FailureCodes), pq.Array(t.SuccessfulPatterns),
		pq.Array(t.ToolsUsed), pq.Array(t.SkillsMatched), t.RequirementSummary)
	return err
}

func (s *PGTrajectoryStore) ListByProject(ctx context.Context, projectID string) ([]domain.TrajectoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE project_id = $1 ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrajectories(rows)
}

func (s *PGTrajectoryStore) GetByPipeline(ctx context.Context, pipelineID string) (*domain.TrajectoryRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE pipeline_id = $1
	`, pipelineID)
	r := &domain.TrajectoryRecord{}
	err := row.Scan(&r.ID, &r.ProjectID, &r.PipelineID, pq.Array(&r.StageSequence),
		&r.TotalChatRounds, &r.TotalTokens, &r.BacktrackCount, &r.RejectionCount,
		pq.Array(&r.FailureCodes), pq.Array(&r.SuccessfulPatterns),
		pq.Array(&r.ToolsUsed), pq.Array(&r.SkillsMatched), &r.RequirementSummary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (s *PGTrajectoryStore) SimilarPatterns(ctx context.Context, projectID string, failureCodes []string, topK int) ([]domain.TrajectoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE project_id = $1 AND failure_codes && $2
		ORDER BY created_at DESC LIMIT $3
	`, projectID, pq.Array(failureCodes), topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrajectories(rows)
}

func (s *PGTrajectoryStore) SuccessfulTools(ctx context.Context, projectID string, stage string, topK int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT unnest(tools_used) AS tool, COUNT(*) as c
		FROM trajectory
		WHERE project_id = $1 AND failure_codes = '{}'
		GROUP BY tool ORDER BY c DESC LIMIT $2
	`, projectID, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var tool string
		var count int
		rows.Scan(&tool, &count)
		result = append(result, tool)
	}
	return result, rows.Err()
}

func (s *PGTrajectoryStore) MatchedSkills(ctx context.Context, projectID string, requirement string, topK int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT unnest(skills_matched) AS skill, COUNT(*) as c
		FROM trajectory
		WHERE project_id = $1 AND skills_matched IS NOT NULL
		GROUP BY skill ORDER BY c DESC LIMIT $2
	`, projectID, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var skill string
		var count int
		rows.Scan(&skill, &count)
		result = append(result, skill)
	}
	return result, rows.Err()
}

func scanTrajectories(rows *sql.Rows) ([]domain.TrajectoryRecord, error) {
	var result []domain.TrajectoryRecord
	for rows.Next() {
		var r domain.TrajectoryRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.PipelineID, pq.Array(&r.StageSequence),
			&r.TotalChatRounds, &r.TotalTokens, &r.BacktrackCount, &r.RejectionCount,
			pq.Array(&r.FailureCodes), pq.Array(&r.SuccessfulPatterns),
			pq.Array(&r.ToolsUsed), pq.Array(&r.SkillsMatched), &r.RequirementSummary); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

var _ domain.TrajectoryStore = (*PGTrajectoryStore)(nil)
