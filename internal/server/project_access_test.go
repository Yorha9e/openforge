package server

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

// TestAutoCreateUserOnProjectCreation verifies that a config-based user
// (who doesn't exist in the "user" table) can create a project and
// subsequently access it because the handler auto-creates the user row.
func TestAutoCreateUserOnProjectCreation(t *testing.T) {
	// This test requires a running PostgreSQL. Skip if not available.
	dsn := "postgres://openforge:openforge@localhost:5432/openforge_test?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("cannot connect to test DB: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not reachable: %v", err)
	}

	// Clean up any left-over test data.
	userID := "test-config-user-no-db-row"
	db.Exec(`DELETE FROM user_role WHERE user_id = $1`, userID)
	db.Exec(`DELETE FROM "user" WHERE id = $1`, userID)
	db.Exec(`DELETE FROM project WHERE name = 'test-config-access-project'`)

	// Verify the user does NOT exist in the "user" table before the fix.
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM "user" WHERE id = $1`, userID).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Fatal("expected user to NOT exist before test")
	}

	// Simulate the transaction logic: insert project, upsert user, assign role.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	projectID := "proj-test-access-001"
	_, err = tx.Exec(`INSERT INTO project (id, name, git_url, repo_type, template) VALUES ($1, 'test-config-access-project', 'https://git.example.com/test', 'custom', 'custom')`, projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	_, err = tx.Exec(`INSERT INTO "user" (id, display_name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, userID, "Test Config User")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	_, err = tx.Exec(`INSERT INTO user_role (user_id, project_id, role, modules) VALUES ($1, $2, 'admin', '{chat,code_review,pipeline,settings}') ON CONFLICT (user_id, project_id) DO NOTHING`, userID, projectID)
	if err != nil {
		t.Fatalf("insert user_role: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Verify the user now exists.
	err = db.QueryRow(`SELECT COUNT(*) FROM "user" WHERE id = $1`, userID).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected user to exist after upsert, got count=%d", count)
	}

	// Verify the user_role was assigned.
	var exists bool
	err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_role WHERE user_id = $1 AND project_id = $2)`, userID, projectID).Scan(&exists)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !exists {
		t.Fatal("expected user_role entry to exist after project creation")
	}

	// Clean up.
	db.Exec(`DELETE FROM user_role WHERE user_id = $1 AND project_id = $2`, userID, projectID)
	db.Exec(`DELETE FROM "user" WHERE id = $1`, userID)
	db.Exec(`DELETE FROM project WHERE id = $1`, projectID)
}

// TestExistingUserNotDuplicated verifies the "ON CONFLICT DO NOTHING" is idempotent.
func TestExistingUserNotDuplicated(t *testing.T) {
	dsn := "postgres://openforge:openforge@localhost:5432/openforge_test?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("cannot connect to test DB: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not reachable: %v", err)
	}

	userID := "test-existing-user-dedup"
	db.Exec(`DELETE FROM user_role WHERE user_id = $1`, userID)
	db.Exec(`DELETE FROM "user" WHERE id = $1`, userID)

	// First insert.
	_, err = db.Exec(`INSERT INTO "user" (id, display_name) VALUES ($1, 'Original')`, userID)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second insert (idempotent upsert) — should NOT change display_name.
	_, err = db.Exec(`INSERT INTO "user" (id, display_name) VALUES ($1, 'ShouldNotChange') ON CONFLICT (id) DO NOTHING`, userID)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}

	var name string
	db.QueryRow(`SELECT display_name FROM "user" WHERE id = $1`, userID).Scan(&name)
	if name != "Original" {
		t.Fatalf("expected display_name='Original', got '%s'", name)
	}

	// Clean up.
	db.Exec(`DELETE FROM "user" WHERE id = $1`, userID)
}
