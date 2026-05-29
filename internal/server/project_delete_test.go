package server

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestProjectDeleteCascade verifies that soft-deleting a project also
// soft-deletes all its pipelines, and that hard-cleanup removes everything.
func TestProjectDeleteCascade(t *testing.T) {
	dsn := "postgres://openforge:openforge@localhost:5432/openforge_test?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("cannot connect to test DB: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("test DB not reachable: %v", err)
	}

	ctx := context.Background()
	projectID := "proj-test-delete-001"
	pipelineID1 := "pipe-test-delete-001"
	pipelineID2 := "pipe-test-delete-002"

	// Clean up any leftover test data
	cleanupTestData(db, projectID, pipelineID1, pipelineID2)

	// Setup: create project + pipelines
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = tx.Exec(`INSERT INTO project (id, name, git_url, repo_type, template)
		VALUES ($1, 'test-delete-project', 'https://git.example.com/test', 'custom', 'custom')`, projectID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert project: %v", err)
	}
	_, err = tx.Exec(`INSERT INTO pipeline (id, project_id, title, status, level)
		VALUES ($1, $2, 'test pipeline 1', 'running', 'basic')`, pipelineID1, projectID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert pipeline 1: %v", err)
	}
	_, err = tx.Exec(`INSERT INTO pipeline (id, project_id, title, status, level)
		VALUES ($1, $2, 'test pipeline 2', 'completed', 'basic')`, pipelineID2, projectID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert pipeline 2: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Verify initial state: project exists, not deleted
	var deletedAt sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT deleted_at FROM project WHERE id = $1`, projectID).Scan(&deletedAt)
	if err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("query initial: %v", err)
	}
	if deletedAt.Valid {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected project not to be deleted initially")
	}

	// Act: soft-delete the project (cascade pipelines)
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = tx2.Exec(`UPDATE pipeline SET deleted_at = NOW(), updated_at = NOW()
		WHERE project_id = $1 AND deleted_at IS NULL`, projectID)
	if err != nil {
		tx2.Rollback()
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("soft delete pipelines: %v", err)
	}
	_, err = tx2.Exec(`UPDATE project SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if err != nil {
		tx2.Rollback()
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("soft delete project: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("commit tx2: %v", err)
	}

	// Assert: project is soft-deleted
	err = db.QueryRowContext(ctx, `SELECT deleted_at FROM project WHERE id = $1`, projectID).Scan(&deletedAt)
	if err != nil || !deletedAt.Valid {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected project to be soft-deleted after delete operation")
	}

	// Assert: pipelines are soft-deleted (cascade)
	var pipe1Deleted sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT deleted_at FROM pipeline WHERE id = $1`, pipelineID1).Scan(&pipe1Deleted)
	if err != nil || !pipe1Deleted.Valid {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected pipeline 1 to be soft-deleted via cascade")
	}

	var pipe2Deleted sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT deleted_at FROM pipeline WHERE id = $1`, pipelineID2).Scan(&pipe2Deleted)
	if err != nil || !pipe2Deleted.Valid {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected pipeline 2 to be soft-deleted via cascade")
	}

	// Act: simulate 30-day retention passing — set deleted_at to 31 days ago
	tx3, err := db.BeginTx(ctx, nil)
	if err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("begin tx3: %v", err)
	}
	_, err = tx3.Exec(`UPDATE project SET deleted_at = NOW() - INTERVAL '31 days' WHERE id = $1`, projectID)
	if err != nil {
		tx3.Rollback()
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("set 31 days ago project: %v", err)
	}
	_, err = tx3.Exec(`UPDATE pipeline SET deleted_at = NOW() - INTERVAL '31 days' WHERE project_id = $1`, projectID)
	if err != nil {
		tx3.Rollback()
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("set 31 days ago pipelines: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("commit tx3: %v", err)
	}

	// Run cleanup
	cleaner := NewSoftDeleteCleaner(db)
	cleaner.cleanup()

	// Assert: project is hard-deleted
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project WHERE id = $1`, projectID).Scan(&count)
	if err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("count project: %v", err)
	}
	if count != 0 {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected project to be hard-deleted after 30-day retention")
	}

	// Assert: pipelines are hard-deleted
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline WHERE project_id = $1`, projectID).Scan(&count)
	if err != nil {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatalf("count pipelines: %v", err)
	}
	if count != 0 {
		cleanupTestData(db, projectID, pipelineID1, pipelineID2)
		t.Fatal("expected all pipelines to be hard-deleted after 30-day retention")
	}

	// Cleanup remaining test data
	cleanupTestData(db, projectID, pipelineID1, pipelineID2)
}

func cleanupTestData(db *sql.DB, projectID, pipelineID1, pipelineID2 string) {
	ctx := context.Background()
	db.ExecContext(ctx, `DELETE FROM conversation_message WHERE pipeline_id IN ($1, $2)`, pipelineID1, pipelineID2)
	db.ExecContext(ctx, `DELETE FROM conversation_branch WHERE pipeline_id IN ($1, $2)`, pipelineID1, pipelineID2)
	db.ExecContext(ctx, `DELETE FROM checkpoint WHERE pipeline_id IN ($1, $2)`, pipelineID1, pipelineID2)
	db.ExecContext(ctx, `DELETE FROM pipeline_stage WHERE pipeline_id IN ($1, $2)`, pipelineID1, pipelineID2)
	db.ExecContext(ctx, `DELETE FROM gate_event WHERE pipeline_id IN ($1, $2)`, pipelineID1, pipelineID2)
	db.ExecContext(ctx, `DELETE FROM pipeline WHERE project_id = $3`, projectID)
	db.ExecContext(ctx, `DELETE FROM file_lock WHERE project_id = $1`, projectID)
	db.ExecContext(ctx, `DELETE FROM user_role WHERE project_id = $1`, projectID)
	db.ExecContext(ctx, `DELETE FROM module_ownership WHERE project_id = $1`, projectID)
	db.ExecContext(ctx, `DELETE FROM token_usage WHERE project_id = $1`, projectID)
	db.ExecContext(ctx, `DELETE FROM cost_quota WHERE project_id = $1`, projectID)
	db.ExecContext(ctx, `DELETE FROM project WHERE id = $1`, projectID)
	// Wait a bit for cleanup to take effect
	time.Sleep(100 * time.Millisecond)
}
