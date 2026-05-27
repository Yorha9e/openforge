package featureflags

import (
	"context"
	"database/sql"
	"os"
	"testing"
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
	// Second seed with different values should NOT overwrite (ON CONFLICT DO NOTHING).
	if err := store.SeedDefaults(ctx, &FeatureFlags{}); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.ComplianceSuite || !flags.ProductionOps {
		t.Error("SeedDefaults overwrote existing — idempotency broken")
	}
}

func TestStore_SaveAll_Transactional(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	f := &FeatureFlags{
		EnterprisePlatform: true, ComplianceSuite: true,
		ProductionOps: false, DistributionArtifacts: false,
	}
	if err := store.SaveAll(ctx, f); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.EnterprisePlatform || !flags.ComplianceSuite {
		t.Error("SaveAll did not persist all flags")
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
		t.Error("overwrite should set to false")
	}
}
