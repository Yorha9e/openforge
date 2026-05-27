package featureflags

import (
	"context"
	"database/sql"
	"fmt"
)

// Store persists and retrieves feature flag overrides from the feature_flags table.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Load reads all flag rows from the DB and returns a FeatureFlags struct.
// Flags not present in the DB are left at their zero value (caller should merge
// with YAML defaults).
func (s *Store) Load(ctx context.Context) (*FeatureFlags, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT flag_key, enabled FROM feature_flags`)
	if err != nil {
		return nil, fmt.Errorf("featureflags load: %w", err)
	}
	defer rows.Close()

	result := Defaults()
	for rows.Next() {
		var key string
		var enabled bool
		if err := rows.Scan(&key, &enabled); err != nil {
			return nil, fmt.Errorf("featureflags scan: %w", err)
		}
		switch key {
		case "enterprise_platform":
			result.EnterprisePlatform = enabled
		case "compliance_suite":
			result.ComplianceSuite = enabled
		case "production_ops":
			result.ProductionOps = enabled
		case "distribution_artifacts":
			result.DistributionArtifacts = enabled
		}
	}
	return result, rows.Err()
}

// Save upserts a single flag value into the DB.
func (s *Store) Save(ctx context.Context, key string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feature_flags (flag_key, enabled, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (flag_key) DO UPDATE SET enabled = $2, updated_at = NOW()`,
		key, enabled)
	if err != nil {
		return fmt.Errorf("featureflags save %s=%v: %w", key, enabled, err)
	}
	return nil
}

// SaveAll persists all 4 flags in a single transaction (G7: avoids partial-update).
func (s *Store) SaveAll(ctx context.Context, f *FeatureFlags) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("featureflags SaveAll begin: %w", err)
	}
	defer tx.Rollback()

	entries := map[string]bool{
		"enterprise_platform":    f.EnterprisePlatform,
		"compliance_suite":        f.ComplianceSuite,
		"production_ops":          f.ProductionOps,
		"distribution_artifacts":  f.DistributionArtifacts,
	}
	for key, enabled := range entries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO feature_flags (flag_key, enabled, updated_at)
			 VALUES ($1, $2, NOW())
			 ON CONFLICT (flag_key) DO UPDATE SET enabled = $2, updated_at = NOW()`,
			key, enabled)
		if err != nil {
			return fmt.Errorf("featureflags SaveAll %s: %w", key, err)
		}
	}
	return tx.Commit()
}

// SeedDefaults writes the YAML-default flags to the DB (idempotent —
// uses ON CONFLICT DO NOTHING so existing user overrides are preserved).
func (s *Store) SeedDefaults(ctx context.Context, defaults *FeatureFlags) error {
	entries := map[string]bool{
		"enterprise_platform":    defaults.EnterprisePlatform,
		"compliance_suite":        defaults.ComplianceSuite,
		"production_ops":          defaults.ProductionOps,
		"distribution_artifacts":  defaults.DistributionArtifacts,
	}
	for key, enabled := range entries {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO feature_flags (flag_key, enabled, updated_at)
			 VALUES ($1, $2, NOW())
			 ON CONFLICT (flag_key) DO NOTHING`,
			key, enabled)
		if err != nil {
			return fmt.Errorf("featureflags seed %s: %w", key, err)
		}
	}
	return nil
}
