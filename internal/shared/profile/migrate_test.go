package profile

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationRunner_Run(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Drop test table if exists from previous run
	db.Exec("DROP TABLE IF EXISTS _test_migration CASCADE")
	db.Exec("DROP TABLE IF EXISTS schema_migrations CASCADE")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "001_test.up.sql"), []byte(`
		CREATE TABLE IF NOT EXISTS _test_migration (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		);
	`), 0644)
	os.WriteFile(filepath.Join(dir, "002_test.up.sql"), []byte(`
		INSERT INTO _test_migration (name) VALUES ('hello');
	`), 0644)

	runner := NewMigrationRunner(db, dir)
	if err := runner.Run(t.Context()); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 migrations tracked, got %d", count)
	}

	// Idempotent: re-run should be no-op
	if err := runner.Run(t.Context()); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if count != 2 {
		t.Errorf("expected still 2 after re-run, got %d", count)
	}
}
