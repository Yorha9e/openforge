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
	changedFilesJSON, _ := json.Marshal(p.ChangedFiles)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pipeline (id, project_id, title, level, status, current_stage, created_by, config, changed_files)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, p.ID, p.ProjectID, p.Title, p.Level, p.Status, p.CurrentStage, p.CreatedBy, stagesJSON, changedFilesJSON)
	return err
}

func (r *PGRepository) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	var p domain.Pipeline
	var config []byte
	var changedFiles []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, level, status, current_stage, created_by,
		       backtrack_count, version, created_at, updated_at, config, changed_files
		FROM pipeline WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.ProjectID, &p.Title, &p.Level, &p.Status,
		&p.CurrentStage, &p.CreatedBy, &p.BacktrackCount, &p.Version,
		&p.CreatedAt, &p.UpdatedAt, &config, &changedFiles)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(config, &p.Stages)
	if changedFiles != nil {
		json.Unmarshal(changedFiles, &p.ChangedFiles)
	}
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
	if err := rows.Err(); err != nil {
		return nil, err
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

func (r *PGRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pipeline SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// --- GateRepository ---

func (r *PGRepository) GetLatestHash(ctx context.Context, pipelineID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, `
		SELECT content_hash FROM gate_event
		WHERE pipeline_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, pipelineID).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

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
	rows, err := r.db.QueryContext(ctx, `
		SELECT TO_CHAR(created_at::date, 'YYYY-MM-DD') AS day, project_id, provider, model,
		       COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0),
		       COALESCE(SUM(estimated_cost), 0)
		FROM token_usage
		WHERE project_id = $1 AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY day, project_id, provider, model
		ORDER BY day DESC
	`, projectID, days)
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
	rows, err := r.db.QueryContext(ctx, `
		SELECT '' AS day, project_id, provider, model,
		       COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0),
		       COALESCE(SUM(estimated_cost), 0)
		FROM token_usage
		WHERE project_id = $1 AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY project_id, provider, model
		ORDER BY COALESCE(SUM(estimated_cost), 0) DESC
	`, projectID, days)
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
		SELECT project_id, token_limit, token_used
		FROM cost_quota
		WHERE project_id = $1 AND month = TO_CHAR(NOW(), 'YYYY-MM')
	`, projectID).Scan(&b.ProjectID, &b.MonthlyLimit, &b.CurrentUsage)
	if err == sql.ErrNoRows {
		return &port.ProjectBudget{
			ProjectID:    projectID,
			MonthlyLimit: 50000000,
			CostLimit:    500.0,
			ResetAt:      nextMonthReset(),
		}, nil
	}
	b.CostLimit = 500.0
	b.ResetAt = nextMonthReset()
	_, currentCost, usageErr := r.GetCurrentMonthUsage(ctx, projectID)
	if usageErr != nil {
		return nil, usageErr
	}
	b.CurrentCost = currentCost
	return &b, err
}

func (r *PGRepository) GetCurrentMonthUsage(ctx context.Context, projectID string) (int64, float64, error) {
	var tokens int64
	var cost float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0),
		       COALESCE(SUM(estimated_cost), 0)
		FROM token_usage
		WHERE project_id = $1 AND created_at >= date_trunc('month', NOW())
	`, projectID).Scan(&tokens, &cost)
	return tokens, cost, err
}

// --- RecordTokenUsage ---

// RecordTokenUsage 写入单条 token_usage 记录。
// 注：DB 中 id 列为 BIGSERIAL（与 created_at 组成复合主键，无单独 UNIQUE 约束）；
//     因此不使用 ON CONFLICT，调用方需自行保证幂等。rec.ID 仅用于日志/回执关联。
func (r *PGRepository) RecordTokenUsage(ctx context.Context, rec port.TokenUsageRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("pipeline repository database is not configured")
	}
	const q = `
		INSERT INTO token_usage
			(pipeline_id, project_id, provider, model,
			 prompt_tokens, completion_tokens, estimated_cost, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, q,
		rec.PipelineID, rec.ProjectID, rec.Provider, rec.Model,
		rec.PromptTokens, rec.CompletionTokens, rec.EstimatedCost, rec.CreatedAt)
	return err
}

// BatchRecordTokenUsage 在单个事务内批量写入；空切片为 no-op。
func (r *PGRepository) BatchRecordTokenUsage(ctx context.Context, recs []port.TokenUsageRecord) error {
	if len(recs) == 0 {
		return nil
	}
	if r == nil || r.db == nil {
		return fmt.Errorf("pipeline repository database is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // safe; Commit 后 no-op

	const q = `
		INSERT INTO token_usage
			(pipeline_id, project_id, provider, model,
			 prompt_tokens, completion_tokens, estimated_cost, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, rec := range recs {
		if _, err := stmt.ExecContext(ctx,
			rec.PipelineID, rec.ProjectID, rec.Provider, rec.Model,
			rec.PromptTokens, rec.CompletionTokens, rec.EstimatedCost, rec.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Cost Quota ---

// GetBudget 读 cost_quota 中给定 project 的月度预算（美元）。0 表示无限制。
//
// TODO(Path-A schema gap): 现有 cost_quota schema 没有 monthly_usd 列，PK 也不是
// project_id 单列。T10 集成验证时需要先补迁移（012_cost_quota_monthly_usd.up.sql）。
func (r *PGRepository) GetBudget(ctx context.Context, projectID string) (float64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("pipeline repository database is not configured")
	}
	var b sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `SELECT monthly_usd FROM cost_quota WHERE project_id = $1`, projectID).Scan(&b)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !b.Valid {
		return 0, nil
	}
	return b.Float64, nil
}

// SetBudget 覆盖写 cost_quota 行（PK = project_id）。monthlyUSD=0 表示清空。
//
// TODO(Path-A schema gap): 现有 cost_quota schema 没有 monthly_usd / updated_at 列，
// 且 UNIQUE 约束是 (project_id, month) 而非 project_id 单列。T10 集成验证时需要
// 先补迁移并决定是用 (project_id) 单列还是 (project_id, month='all') 行。
func (r *PGRepository) SetBudget(ctx context.Context, projectID string, monthlyUSD float64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("pipeline repository database is not configured")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cost_quota (project_id, monthly_usd, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (project_id) DO UPDATE SET monthly_usd = EXCLUDED.monthly_usd, updated_at = NOW()
	`, projectID, monthlyUSD)
	return err
}

func nextMonthReset() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
}

var _ port.ConversationRepository = (*PGRepository)(nil)

// --- ConversationRepository ---

func (r *PGRepository) SaveMessage(ctx context.Context, msg *port.DBMessage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversation_message (pipeline_id, branch_id, msg_seq, role, msg_type, content, token_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (pipeline_id, branch_id, msg_seq) DO UPDATE SET
			content = EXCLUDED.content,
			token_count = EXCLUDED.token_count
	`, msg.PipelineID, msg.BranchID, msg.MsgSeq, msg.Role, msg.MsgType, msg.Content, msg.TokenCount)
	return err
}

func (r *PGRepository) GetMessages(ctx context.Context, pipelineID string, branchID string) ([]*port.DBMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pipeline_id, branch_id, msg_seq, role, msg_type, content, COALESCE(token_count, 0), created_at
		FROM conversation_message
		WHERE pipeline_id = $1 AND branch_id = $2
		ORDER BY msg_seq ASC
	`, pipelineID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*port.DBMessage
	for rows.Next() {
		var m port.DBMessage
		if err := rows.Scan(&m.ID, &m.PipelineID, &m.BranchID, &m.MsgSeq, &m.Role, &m.MsgType, &m.Content, &m.TokenCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *PGRepository) CreateBranch(ctx context.Context, b *port.DBBranch) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversation_branch (id, pipeline_id, parent_branch, fork_msg_seq, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, b.ID, b.PipelineID, b.ParentBranch, b.ForkMsgSeq, b.Status, b.CreatedBy)
	return err
}

func (r *PGRepository) GetBranch(ctx context.Context, branchID string) (*port.DBBranch, error) {
	var b port.DBBranch
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, parent_branch, fork_msg_seq, status, created_by, created_at
		FROM conversation_branch WHERE id = $1
	`, branchID).Scan(&b.ID, &b.PipelineID, &b.ParentBranch, &b.ForkMsgSeq, &b.Status, &b.CreatedBy, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (r *PGRepository) GetActiveBranch(ctx context.Context, pipelineID string) (*port.DBBranch, error) {
	var b port.DBBranch
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, parent_branch, fork_msg_seq, status, created_by, created_at
		FROM conversation_branch
		WHERE pipeline_id = $1 AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, pipelineID).Scan(&b.ID, &b.PipelineID, &b.ParentBranch, &b.ForkMsgSeq, &b.Status, &b.CreatedBy, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (r *PGRepository) ListBranches(ctx context.Context, pipelineID string) ([]*port.DBBranch, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pipeline_id, parent_branch, fork_msg_seq, status, created_by, created_at
		FROM conversation_branch
		WHERE pipeline_id = $1
		ORDER BY created_at ASC
	`, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []*port.DBBranch
	for rows.Next() {
		var b port.DBBranch
		if err := rows.Scan(&b.ID, &b.PipelineID, &b.ParentBranch, &b.ForkMsgSeq, &b.Status, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		branches = append(branches, &b)
	}
	if branches == nil {
		branches = []*port.DBBranch{}
	}
	return branches, nil
}

// DeactivateBranch marks a conversation_branch row as no longer the active
// branch for a pipeline. Path C T4: minimal implementation that flips the
// status column. A fuller implementation that also forks the active branch
// to a new "main" belongs to a later task.
func (r *PGRepository) DeactivateBranch(ctx context.Context, branchID string) error {
	if branchID == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE conversation_branch
		SET status = 'inactive'
		WHERE id = $1
	`, branchID)
	return err
}
