package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// PGSettingsRepo persists user settings to PostgreSQL.
type PGSettingsRepo struct {
	db *sql.DB
}

func NewPGSettingsRepo(db *sql.DB) *PGSettingsRepo {
	return &PGSettingsRepo{db: db}
}

// UserSettingsRow is the database representation of user settings.
type UserSettingsRow struct {
	UserID        string
	Notifications json.RawMessage
	Layout        json.RawMessage
	Language      json.RawMessage
	Project       json.RawMessage
}

// Get retrieves settings for a user. Returns nil if not found.
func (r *PGSettingsRepo) Get(ctx context.Context, userID string) (*UserSettingsRow, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, notifications, layout, language, project
		 FROM user_settings WHERE user_id = $1`, userID)
	var s UserSettingsRow
	if err := row.Scan(&s.UserID, &s.Notifications, &s.Layout, &s.Language, &s.Project); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return &s, nil
}

// Upsert creates or updates settings for a user.
func (r *PGSettingsRepo) Upsert(ctx context.Context, s *UserSettingsRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_settings (user_id, notifications, layout, language, project)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE SET
			notifications = EXCLUDED.notifications,
			layout = EXCLUDED.layout,
			language = EXCLUDED.language,
			project = EXCLUDED.project,
			updated_at = now()`,
		s.UserID, s.Notifications, s.Layout, s.Language, s.Project)
	return err
}

// SaveLayout persists a per-user layout configuration as JSON. Path C T4:
// minimal implementation that upserts the layout column on the existing
// user_settings row, falling back to a fresh insert when no row exists.
func (r *PGSettingsRepo) SaveLayout(ctx context.Context, userID string, layout map[string]any) error {
	if userID == "" {
		return nil
	}
	payload, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, layout, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET
			layout = EXCLUDED.layout,
			updated_at = now()
	`, userID, payload)
	return err
}
