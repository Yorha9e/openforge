package featureflags

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=openforge password=openforge dbname=openforge sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("db open failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("db unreachable: %v", err)
	}
	db.Exec("DELETE FROM feature_flags")
	return db
}

func TestStore_SaveAndLoad(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	if err := store.Save(ctx, "enterprise_platform", true); err != nil {
		t.Fatalf("save: %v", err)
	}
	flags, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !flags.EnterprisePlatform {
		t.Error("EnterprisePlatform should be true")
	}
	if flags.ComplianceSuite {
		t.Error("unsaved flag should be false")
	}
}

func TestStore_SeedDefaults_Idempotent(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	defaults := &FeatureFlags{ComplianceSuite: true, ProductionOps: true}
	if err := store.SeedDefaults(ctx, defaults); err != nil {
		t.Fatalf("seed1: %v", err)
	}
	if err := store.SeedDefaults(ctx, &FeatureFlags{}); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.ComplianceSuite || !flags.ProductionOps {
		t.Error("SeedDefaults overwrote existing — idempotency broken")
	}
}

func TestStore_Save_Overwrite(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	store.Save(ctx, "distribution_artifacts", true)
	store.Save(ctx, "distribution_artifacts", false)
	flags, _ := store.Load(ctx)
	if flags.DistributionArtifacts {
		t.Error("overwrite should have set false")
	}
}

// T10: SaveAll persists lifecycle fields (owner, status, rollout_pct,
// expires_at) alongside the legacy boolean column.
func TestStore_SaveAll_PersistsLifecycle(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	ff := &FeatureFlags{
		EnterprisePlatform:    true,
		ComplianceSuite:       true,
		ProductionOps:         false,
		DistributionArtifacts: true,
		Lifecycle: map[string]FlagLifecycle{
			"enterprise_platform": {
				Owner:      "platform-team",
				Status:     StatusBeta,
				RolloutPct: 25,
				ExpiresAt:  &expires,
			},
		},
	}
	if err := store.SaveAll(ctx, ff); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}

	// Verify the lifecycle row was written by reading it back directly.
	var status string
	var rolloutPct int
	var gotExpires *time.Time
	var owner string
	err := db.QueryRowContext(ctx,
		`SELECT owner, status, rollout_pct, expires_at
		 FROM feature_flags WHERE flag_key = $1`,
		"enterprise_platform").Scan(&owner, &status, &rolloutPct, &gotExpires)
	if err != nil {
		t.Fatalf("scan lifecycle row: %v", err)
	}
	if owner != "platform-team" {
		t.Errorf("owner = %q, want %q", owner, "platform-team")
	}
	if status != StatusBeta {
		t.Errorf("status = %q, want %q", status, StatusBeta)
	}
	if rolloutPct != 25 {
		t.Errorf("rollout_pct = %d, want 25", rolloutPct)
	}
	if gotExpires == nil || !gotExpires.Equal(expires) {
		t.Errorf("expires_at = %v, want %v", gotExpires, expires)
	}
}