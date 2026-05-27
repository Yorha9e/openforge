package adapter

import (
	"context"
	"database/sql"
	"time"

	"openforge/internal/agent/domain"
)

// PGExperimentStore implements domain.ExperimentStore backed by PostgreSQL.
type PGExperimentStore struct {
	db *sql.DB
}

// NewPGExperimentStore creates a new PGExperimentStore.
func NewPGExperimentStore(db *sql.DB) *PGExperimentStore {
	return &PGExperimentStore{db: db}
}

// Create inserts a new A/B experiment into ab_experiment.
func (s *PGExperimentStore) Create(ctx context.Context, exp *domain.ABExperiment) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ab_experiment (id, knowledge_id, cohort_a_ratio, status)
		VALUES ($1, $2, $3, $4)
	`, exp.ID, exp.KnowledgeID, exp.CohortARatio, exp.Status)
	return err
}

// Get retrieves an A/B experiment by ID.
func (s *PGExperimentStore) Get(ctx context.Context, id string) (*domain.ABExperiment, error) {
	exp := &domain.ABExperiment{}
	var startedAt, completedAt sql.NullTime
	var verdict, pValue, effectSize sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, knowledge_id, cohort_a_ratio, status,
			COALESCE(verdict, ''), COALESCE(p_value, 0), COALESCE(effect_size, 0),
			started_at, completed_at
		FROM ab_experiment WHERE id = $1
	`, id).Scan(
		&exp.ID, &exp.KnowledgeID, &exp.CohortARatio, &exp.Status,
		&verdict, &pValue, &effectSize,
		&startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	exp.Verdict = verdict.String
	exp.StartedAt = startedAt.Time
	if completedAt.Valid {
		exp.CompletedAt = completedAt.Time
	}
	return exp, nil
}

// Assign records a pipeline's cohort assignment in ab_experiment_assignment.
func (s *PGExperimentStore) Assign(ctx context.Context, a *domain.ABExperimentAssignment) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ab_experiment_assignment (experiment_id, pipeline_id, cohort)
		VALUES ($1, $2, $3)
	`, a.ExperimentID, a.PipelineID, a.Cohort)
	return err
}

// Complete finalizes an experiment with statistical results.
func (s *PGExperimentStore) Complete(ctx context.Context, id, verdict string, pValue, effectSize float64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ab_experiment
		SET status = 'completed', verdict = $2, p_value = $3, effect_size = $4, completed_at = $5
		WHERE id = $1 AND status = 'running'
	`, id, verdict, pValue, effectSize, time.Now().UTC())
	return err
}

// ListActive returns all currently running experiments.
func (s *PGExperimentStore) ListActive(ctx context.Context) ([]*domain.ABExperiment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, knowledge_id, cohort_a_ratio, status,
			COALESCE(verdict, ''), COALESCE(p_value, 0), COALESCE(effect_size, 0),
			started_at, completed_at
		FROM ab_experiment WHERE status = 'running'
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*domain.ABExperiment
	for rows.Next() {
		exp := &domain.ABExperiment{}
		var startedAt, completedAt sql.NullTime
		var verdict sql.NullString
		var pValue, effectSize sql.NullFloat64
		if err := rows.Scan(
			&exp.ID, &exp.KnowledgeID, &exp.CohortARatio, &exp.Status,
			&verdict, &pValue, &effectSize,
			&startedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		exp.Verdict = verdict.String
		if pValue.Valid {
			exp.PValue = pValue.Float64
		}
		if effectSize.Valid {
			exp.EffectSize = effectSize.Float64
		}
		exp.StartedAt = startedAt.Time
		if completedAt.Valid {
			exp.CompletedAt = completedAt.Time
		}
		results = append(results, exp)
	}
	return results, rows.Err()
}

// AssignCohort is a convenience method that assigns a pipeline to cohort A or B
// for a given experiment. It randomizes based on the experiment's cohort_a_ratio.
func (s *PGExperimentStore) AssignCohort(ctx context.Context, experimentID, pipelineID, cohort string) error {
	return s.Assign(ctx, &domain.ABExperimentAssignment{
		ExperimentID: experimentID,
		PipelineID:   pipelineID,
		Cohort:       cohort,
	})
}

// GetAssignment retrieves the cohort assignment for a pipeline.
func (s *PGExperimentStore) GetAssignment(ctx context.Context, pipelineID string) (*domain.ABExperimentAssignment, error) {
	a := &domain.ABExperimentAssignment{}
	err := s.db.QueryRowContext(ctx, `
		SELECT experiment_id, pipeline_id, cohort
		FROM ab_experiment_assignment WHERE pipeline_id = $1
	`, pipelineID).Scan(&a.ExperimentID, &a.PipelineID, &a.Cohort)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// GetCohortResults returns code acceptance rates grouped by cohort for a given experiment.
func (s *PGExperimentStore) GetCohortResults(ctx context.Context, experimentID string) (cohortA []float64, cohortB []float64, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cohort, COALESCE(code_acceptance_rate, 0)
		FROM ab_experiment_assignment
		WHERE experiment_id = $1 AND code_acceptance_rate IS NOT NULL
	`, experimentID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c string
		var rate float64
		if err := rows.Scan(&c, &rate); err != nil {
			return nil, nil, err
		}
		switch c {
		case "A":
			cohortA = append(cohortA, rate)
		case "B":
			cohortB = append(cohortB, rate)
		}
	}
	return cohortA, cohortB, rows.Err()
}

// compile-time interface check
var _ domain.ExperimentStore = (*PGExperimentStore)(nil)
