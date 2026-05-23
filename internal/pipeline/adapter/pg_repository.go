package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

var _ port.TokenCostRepository = (*PGRepository)(nil)

type PGRepository struct {
	db *sql.DB
}

func NewPGRepository(db *sql.DB) *PGRepository {
	return &PGRepository{db: db}
}

var _ port.PipelineRepository = (*PGRepository)(nil)
var _ port.GateRepository = (*PGRepository)(nil)

// --- PipelineRepository ---

func (r *PGRepository) Create(ctx context.Context, p *domain.Pipeline) error {
	stagesJSON, _ := json.Marshal(p.Stages)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pipeline (id, project_id, title, level, status, current_stage, created_by, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, p.ID, p.ProjectID, p.Title, p.Level, p.Status, p.CurrentStage, p.CreatedBy, stagesJSON)
	return err
}

func (r *PGRepository) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	var p domain.Pipeline
	var config []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, level, status, current_stage, created_by,
		       backtrack_count, version, created_at, updated_at, config
		FROM pipeline WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.ProjectID, &p.Title, &p.Level, &p.Status,
		&p.CurrentStage, &p.CreatedBy, &p.BacktrackCount, &p.Version,
		&p.CreatedAt, &p.UpdatedAt, &config)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(config, &p.Stages)
	return &p, nil
}

func (r *PGRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, title, level, status, current_stage, created_by,
		       backtrack_count, version, created_at
		FROM pipeline WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Pipeline
	for rows.Next() {
		var p domain.Pipeline
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Title, &p.Level, &p.Status,
			&p.CurrentStage, &p.CreatedBy, &p.BacktrackCount, &p.Version, &p.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &p)
	}
	return result, nil
}

func (r *PGRepository) UpdateStatus(ctx context.Context, id string, status string, version int) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE pipeline SET status = $2, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND version = $3 AND deleted_at IS NULL
	`, id, status, version)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pipeline %q: optimistic lock conflict (version %d)", id, version)
	}
	return nil
}

func (r *PGRepository) IncrementBacktrack(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pipeline SET backtrack_count = backtrack_count + 1, updated_at = NOW()
		WHERE id = $1 AND backtrack_count < 3
	`, id)
	return err
}

// --- GateRepository ---

func (r *PGRepository) CreateEvent(ctx context.Context, ev *domain.GateEvent) error {
	comments, _ := json.Marshal(ev.LineComments)
	checklist, _ := json.Marshal(ev.Checklist)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gate_event (pipeline_id, stage, event, actor, decision,
			line_comments, summary_feedback, checklist, artifact_hash, prev_hash, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, ev.PipelineID, ev.Stage, ev.Event, ev.Actor, ev.Decision,
		comments, ev.SummaryFeedback, checklist,
		ev.ArtifactHash, ev.PrevHash, ev.ContentHash)
	return err
}

func (r *PGRepository) ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pipeline_id, stage, event, actor, decision,
		       line_comments, summary_feedback, checklist, artifact_hash, created_at
		FROM gate_event WHERE pipeline_id = $1 ORDER BY created_at DESC
	`, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.GateEvent
	for rows.Next() {
		var ev domain.GateEvent
		var comments, checklist []byte
		if err := rows.Scan(&ev.PipelineID, &ev.Stage, &ev.Event, &ev.Actor,
			&ev.Decision, &comments, &ev.SummaryFeedback, &checklist,
			&ev.ArtifactHash, &ev.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(comments, &ev.LineComments)
		json.Unmarshal(checklist, &ev.Checklist)
		events = append(events, &ev)
	}
	return events, nil
}

func (r *PGRepository) ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pipeline_id, stage, event, actor, decision, line_comments,
		       summary_feedback, checklist, artifact_hash, created_at
		FROM gate_event WHERE event = 'awaiting'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.GateEvent
	for rows.Next() {
		var ev domain.GateEvent
		var comments, checklist []byte
		if err := rows.Scan(&ev.PipelineID, &ev.Stage, &ev.Event, &ev.Actor,
			&ev.Decision, &comments, &ev.SummaryFeedback, &checklist,
			&ev.ArtifactHash, &ev.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(comments, &ev.LineComments)
		json.Unmarshal(checklist, &ev.Checklist)
		events = append(events, &ev)
	}
	return events, nil
}

func (r *PGRepository) Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gate_event (pipeline_id, stage, event, actor, prev_hash, content_hash)
		VALUES ($1, $2, 'claimed', $3, 'genesis', 'genesis')
	`, pipelineID, stage, actor)
	return err
}

func (r *PGRepository) ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gate_event SET event = 'awaiting', actor = ''
		WHERE pipeline_id = $1 AND stage = $2 AND event = 'claimed' AND actor = $3
	`, pipelineID, stage, actor)
	return err
}

// --- TokenCostRepository ---

func (r *PGRepository) AggregateByDay(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	query := fmt.Sprintf("SELECT DATE(timestamp) as day, project_id, provider, model, SUM(prompt_tokens), SUM(completion_tokens), SUM(estimated_cost) FROM token_usage WHERE project_id = $1 AND timestamp >= NOW() - INTERVAL '%d days' GROUP BY day, project_id, provider, model ORDER BY day DESC", days)
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []port.TokenCostRow
	for rows.Next() {
		var row port.TokenCostRow
		if err := rows.Scan(&row.Date, &row.ProjectID, &row.Provider, &row.Model,
			&row.PromptTokens, &row.CompletionTokens, &row.EstimatedCost); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func (r *PGRepository) AggregateByModel(ctx context.Context, projectID string, days int) ([]port.TokenCostRow, error) {
	query := fmt.Sprintf("SELECT '' as day, project_id, provider, model, SUM(prompt_tokens), SUM(completion_tokens), SUM(estimated_cost) FROM token_usage WHERE project_id = $1 AND timestamp >= NOW() - INTERVAL '%d days' GROUP BY project_id, provider, model ORDER BY SUM(estimated_cost) DESC", days)
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []port.TokenCostRow
	for rows.Next() {
		var row port.TokenCostRow
		if err := rows.Scan(&row.Date, &row.ProjectID, &row.Provider, &row.Model,
			&row.PromptTokens, &row.CompletionTokens, &row.EstimatedCost); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func (r *PGRepository) GetProjectBudget(ctx context.Context, projectID string) (*port.ProjectBudget, error) {
	var b port.ProjectBudget
	err := r.db.QueryRowContext(ctx, `
		SELECT project_id, COALESCE(token_limit, 50000000), COALESCE(cost_limit_dollars, 500.0),
			COALESCE(current_tokens, 0), COALESCE(current_cost, 0), COALESCE(period_end, NOW())
		FROM cost_quota WHERE project_id = $1
	`, projectID).Scan(&b.ProjectID, &b.MonthlyLimit, &b.CostLimit,
		&b.CurrentUsage, &b.CurrentCost, &b.ResetAt)
	if err == sql.ErrNoRows {
		return &port.ProjectBudget{
			ProjectID:    projectID,
			MonthlyLimit: 50000000,
			CostLimit:    500.0,
			ResetAt:      nextMonthReset(),
		}, nil
	}
	return &b, err
}

func (r *PGRepository) GetCurrentMonthUsage(ctx context.Context, projectID string) (int64, float64, error) {
	var tokens int64
	var cost float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0),
		       COALESCE(SUM(estimated_cost), 0)
		FROM token_usage
		WHERE project_id = $1 AND timestamp >= date_trunc('month', NOW())
	`, projectID).Scan(&tokens, &cost)
	return tokens, cost, err
}

func nextMonthReset() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
}
