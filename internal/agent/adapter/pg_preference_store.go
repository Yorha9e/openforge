package adapter

import (
	"context"
	"database/sql"

	"openforge/internal/agent/domain"
)

type PGPreferenceStore struct {
	db *sql.DB
}

func NewPGPreferenceStore(db *sql.DB) *PGPreferenceStore {
	return &PGPreferenceStore{db: db}
}

func (s *PGPreferenceStore) Upsert(ctx context.Context, pref domain.PreferenceRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO preference (project_id, key, value, weight, source, conflict_count, last_activated)
		VALUES ($1, $2, $3, $4, $5, 1, NOW())
		ON CONFLICT (project_id, key, value) DO UPDATE SET
			weight = EXCLUDED.weight,
			conflict_count = preference.conflict_count + 1,
			last_activated = NOW()
	`, pref.ProjectID, pref.Key, pref.Value, pref.Weight, pref.Source)
	return err
}

func (s *PGPreferenceStore) ListByProject(ctx context.Context, projectID string) ([]domain.PreferenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, key, value, weight, source, conflict_count,
			COALESCE(last_activated::text, '')
		FROM preference WHERE project_id = $1 ORDER BY weight DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PreferenceRecord
	for rows.Next() {
		var r domain.PreferenceRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Key, &r.Value, &r.Weight, &r.Source, &r.ConflictCount, &r.LastActivated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PGPreferenceStore) Get(ctx context.Context, projectID, key string) ([]domain.PreferenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, key, value, weight, source, conflict_count,
			COALESCE(last_activated::text, '')
		FROM preference WHERE project_id = $1 AND key = $2 ORDER BY conflict_count DESC, last_activated DESC
	`, projectID, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PreferenceRecord
	for rows.Next() {
		var r domain.PreferenceRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Key, &r.Value, &r.Weight, &r.Source, &r.ConflictCount, &r.LastActivated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PGPreferenceStore) ResolveConflict(ctx context.Context, projectID, key string) (*domain.PreferenceRecord, error) {
	records, err := s.Get(ctx, projectID, key)
	if err != nil {
		return nil, err
	}
	return domain.ResolveConflict(records), nil
}

var _ domain.PreferenceStore = (*PGPreferenceStore)(nil)
