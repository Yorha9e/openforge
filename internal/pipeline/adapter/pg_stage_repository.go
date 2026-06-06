package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

var _ port.StageRepository = (*PGStageRepository)(nil)

// PGStageRepository implements StageRepository backed by PostgreSQL.
type PGStageRepository struct {
	db *sql.DB
}

// NewPGStageRepository creates a new PGStageRepository.
func NewPGStageRepository(db *sql.DB) *PGStageRepository {
	return &PGStageRepository{db: db}
}

// Create inserts a new pipeline stage record.
func (r *PGStageRepository) Create(ctx context.Context, s *domain.Stage) error {
	if s.ID == "" {
		return fmt.Errorf("stage ID is required")
	}
	if !domain.IsValidStage(s.Type) {
		return fmt.Errorf("invalid stage type: %s", s.Type)
	}
	if !domain.IsValidStageStatus(s.Status) {
		return fmt.Errorf("invalid stage status: %s", s.Status)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pipeline_stage (
			id, pipeline_id, stage, status, requirement_summary, constraints,
			preference_profile, module_index_subset, summary, artifact_ref,
			artifact_hash, schema_version, version, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, s.ID, s.PipelineID, s.Type, s.Status, s.RequirementSummary,
		pqStringArray(s.Constraints), s.PreferenceProfile, s.ModuleIndexSubset,
		s.Summary, s.ArtifactRef, s.ArtifactHash, s.SchemaVersion, s.Version,
		s.StartedAt, s.CompletedAt)
	return err
}

// GetByID retrieves a single stage by its ID.
func (r *PGStageRepository) GetByID(ctx context.Context, id string) (*domain.Stage, error) {
	s := &domain.Stage{}
	var constraints []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, stage, status, requirement_summary, constraints,
		       preference_profile, module_index_subset, summary, artifact_ref,
		       artifact_hash, schema_version, version, started_at, completed_at
		FROM pipeline_stage WHERE id = $1
	`, id).Scan(&s.ID, &s.PipelineID, &s.Type, &s.Status, &s.RequirementSummary,
		&constraints, &s.PreferenceProfile, &s.ModuleIndexSubset, &s.Summary,
		&s.ArtifactRef, &s.ArtifactHash, &s.SchemaVersion, &s.Version,
		&s.StartedAt, &s.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Constraints = parseStringArray(constraints)
	return s, nil
}

// ListByPipeline returns all stages for a given pipeline, ordered by stage sequence.
func (r *PGStageRepository) ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.Stage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pipeline_id, stage, status, requirement_summary, constraints,
		       preference_profile, module_index_subset, summary, artifact_ref,
		       artifact_hash, schema_version, version, started_at, completed_at
		FROM pipeline_stage
		WHERE pipeline_id = $1
		ORDER BY
			CASE stage
				WHEN 'clarify' THEN 1
				WHEN 'decompose' THEN 2
				WHEN 'impl' THEN 3
				WHEN 'test' THEN 4
				WHEN 'deploy' THEN 5
				WHEN 'verify' THEN 6
			END
	`, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stages []*domain.Stage
	for rows.Next() {
		s := &domain.Stage{}
		var constraints []byte
		if err := rows.Scan(&s.ID, &s.PipelineID, &s.Type, &s.Status,
			&s.RequirementSummary, &constraints, &s.PreferenceProfile,
			&s.ModuleIndexSubset, &s.Summary, &s.ArtifactRef, &s.ArtifactHash,
			&s.SchemaVersion, &s.Version, &s.StartedAt, &s.CompletedAt); err != nil {
			return nil, err
		}
		s.Constraints = parseStringArray(constraints)
		stages = append(stages, s)
	}
	return stages, nil
}

// UpdateStatus updates the status, summary, and completed_at timestamp of a stage.
func (r *PGStageRepository) UpdateStatus(ctx context.Context, id string, status string, summary string) error {
	if !domain.IsValidStageStatus(status) {
		return fmt.Errorf("invalid stage status: %s", status)
	}

	var completedAt *time.Time
	if status == "passed" || status == "failed" || status == "skipped" {
		now := time.Now().UTC()
		completedAt = &now
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE pipeline_stage
		SET status = $1, summary = $2, completed_at = $3, version = version + 1
		WHERE id = $4
	`, status, summary, completedAt, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("stage not found: %s", id)
	}
	return nil
}

// pqStringArray converts a Go string slice to a PostgreSQL text array literal.
func pqStringArray(ss []string) string {
	if len(ss) == 0 {
		return "{}"
	}
	result := "{"
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += `"` + s + `"`
	}
	result += "}"
	return result
}

// parseStringArray parses a PostgreSQL text array literal into a Go string slice.
func parseStringArray(data []byte) []string {
	if len(data) == 0 || string(data) == "{}" {
		return nil
	}
	// Simple parser for {a,b,c} format
	s := string(data)
	if s[0] == '{' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '}' {
		s = s[:len(s)-1]
	}
	if s == "" {
		return nil
	}
	var result []string
	current := ""
	inQuote := false
	for _, c := range s {
		switch c {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, current)
				current = ""
				continue
			}
			current += string(c)
		default:
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
