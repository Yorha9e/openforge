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

// SaveAll persists all 4 flags + their lifecycle fields in a single
// transaction (G7: avoids partial-update).
//
// For each of the 4 known flag keys, SaveAll writes both the legacy
// boolean `enabled` column AND the new lifecycle columns (owner, status,
// rollout_pct, expires_at). Unknown keys in the Lifecycle map are also
// upserted, so admins can register custom flags without schema changes.
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

	// Merge known keys with any extra keys present in Lifecycle.
	allKeys := make(map[string]struct{}, len(entries))
	for k := range entries {
		allKeys[k] = struct{}{}
	}
	for k := range f.Lifecycle {
		allKeys[k] = struct{}{}
	}

	for key := range allKeys {
		enabled := entries[key] // false for keys only in Lifecycle
		lc, hasLC := f.Lifecycle[key]
		var (
			owner       string
			status      string
			rolloutPct  int
			expiresAt   interface{}
		)
		if hasLC {
			owner = lc.Owner
			status = lc.Status
			rolloutPct = lc.RolloutPct
			if lc.ExpiresAt != nil {
				expiresAt = *lc.ExpiresAt
			}
		}
		if owner == "" {
			owner = "system"
		}
		if status == "" {
			status = StatusExperimental
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO feature_flags
			 (flag_key, enabled, owner, status, rollout_pct, expires_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW())
			 ON CONFLICT (flag_key) DO UPDATE SET
			     enabled     = EXCLUDED.enabled,
			     owner       = EXCLUDED.owner,
			     status      = EXCLUDED.status,
			     rollout_pct = EXCLUDED.rollout_pct,
			     expires_at  = EXCLUDED.expires_at,
			     updated_at  = NOW()`,
			key, enabled, owner, status, rolloutPct, expiresAt)
		if err != nil {
			return fmt.Errorf("featureflags SaveAll %s: %w", key, err)
		}
	}
	return tx.Commit()
}

// SeedDefaults writes the YAML-default flags to the DB (idempotent —
// uses ON CONFLICT DO NOTHING so existing user overrides are preserved).
// If the feature_flags table does not exist yet (migrations not run), skip silently.
func (s *Store) SeedDefaults(ctx context.Context, defaults *FeatureFlags) error {
	// Check if table exists — skip seeding if migrations haven't been run yet
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'feature_flags'
		)`).Scan(&exists)
	if err != nil || !exists {
		return nil // table doesn't exist yet, migrations haven't run
	}

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
