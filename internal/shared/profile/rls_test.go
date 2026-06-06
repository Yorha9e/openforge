package profile

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

// TestWithProjectID_RoundTrip verifies the context helpers carry a value
// through the context chain and return "" when nothing is set.
func TestWithProjectID_RoundTrip(t *testing.T) {
	if got := ProjectIDFromContext(context.Background()); got != "" {
		t.Errorf("ProjectIDFromContext(empty) = %q, want empty", got)
	}

	ctx := WithProjectID(context.Background(), "pr-1")
	if got := ProjectIDFromContext(ctx); got != "pr-1" {
		t.Errorf("ProjectIDFromContext(stored) = %q, want %q", got, "pr-1")
	}

	// Overwrite.
	ctx2 := WithProjectID(ctx, "pr-2")
	if got := ProjectIDFromContext(ctx2); got != "pr-2" {
		t.Errorf("overwrite: got %q, want pr-2", got)
	}
	// Original ctx should not be mutated.
	if got := ProjectIDFromContext(ctx); got != "pr-1" {
		t.Errorf("original mutated: got %q, want pr-1", got)
	}
}

func TestQuoteLiteral_EscapesQuotes(t *testing.T) {
	if got := quoteLiteral("abc"); got != `'abc'` {
		t.Errorf("quoteLiteral(abc) = %q, want 'abc'", got)
	}
	if got := quoteLiteral("a'b"); got != `'a''b'` {
		t.Errorf("quoteLiteral(a'b) = %q, want 'a''b'", got)
	}
}

// TestRLSPipelineIsolation is the integration test for the 018 migration
// and RLSConn wrapper. It is skipped when TEST_DATABASE_URL is not set so
// CI stays green.
//
// The test:
//  1. Creates two project rows + two pipeline rows (one per project).
//  2. Sets app.current_project_id = pr-1 and verifies only pr-1's pipeline
//     is visible.
//  3. Switches the GUC to pr-2 and verifies only pr-2's pipeline is visible.
//  4. With no project_id set, RLS hides everything (because project_id is
//     NOT NULL on pipeline, the policy evaluating to NULL filters rows out).
func TestRLSPipelineIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Pre-flight: confirm the policy exists. If migration 018 has not been
	// applied, the policy will be missing and the test would silently pass
	// (RLS enabled, no policy = all rows visible). Better to fail loudly.
	var policyName string
	err = db.QueryRowContext(ctx, `
		SELECT policyname FROM pg_policies
		WHERE schemaname = current_schema() AND tablename = 'pipeline'
		  AND policyname = 'pipeline_project_isolation'
	`).Scan(&policyName)
	if err == sql.ErrNoRows {
		t.Fatalf("policy pipeline_project_isolation not found — apply migration 018")
	}
	if err != nil {
		t.Fatalf("lookup policy: %v", err)
	}

	pr1 := "11111111-1111-1111-1111-111111111111"
	pr2 := "22222222-2222-2222-2222-222222222222"
	pl1 := "pl-" + pr1
	pl2 := "pl-" + pr2

	cleanup := func() {
		// Use DB() so cleanup queries bypass RLS (we want to actually
		// delete the rows regardless of project context).
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline WHERE id IN ($1, $2)`, pl1, pl2)
		_, _ = db.ExecContext(ctx, `DELETE FROM project WHERE id IN ($1, $2)`, pr1, pr2)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Seed (also bypasses RLS).
	if _, err := db.ExecContext(ctx, `
		INSERT INTO project (id, name, git_url, repo_type) VALUES
		  ($1, 'rls-test-1', 'https://example.com/r1', 'custom'),
		  ($2, 'rls-test-2', 'https://example.com/r2', 'custom')
	`, pr1, pr2); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO pipeline (id, project_id, title, level, created_by) VALUES
		  ($1, $2, 'p1', 'L1', 'rls@test'),
		  ($3, $4, 'p2', 'L1', 'rls@test')
	`, pl1, pr1, pl2, pr2); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}

	r := NewRLSConn(db)

	// Case 1: pr-1 context → only pl1 visible.
	rows, err := r.QueryContext(WithProjectID(ctx, pr1),
		`SELECT id FROM pipeline WHERE id IN ($1, $2) ORDER BY id`, pl1, pl2)
	if err != nil {
		t.Fatalf("query pr1: %v", err)
	}
	defer rows.Close()
	var seen []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen = append(seen, id)
	}
	if len(seen) != 1 || seen[0] != pl1 {
		t.Errorf("pr1: visible pipelines = %v, want [%s]", seen, pl1)
	}
	rows.Close()

	// Case 2: pr-2 context → only pl2 visible.
	rows, err = r.QueryContext(WithProjectID(ctx, pr2),
		`SELECT id FROM pipeline WHERE id IN ($1, $2) ORDER BY id`, pl1, pl2)
	if err != nil {
		t.Fatalf("query pr2: %v", err)
	}
	defer rows.Close()
	seen = seen[:0]
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen = append(seen, id)
	}
	if len(seen) != 1 || seen[0] != pl2 {
		t.Errorf("pr2: visible pipelines = %v, want [%s]", seen, pl2)
	}
	rows.Close()

	// Case 3: no project_id context → policy evaluates to NULL, no rows.
	rows, err = r.QueryContext(ctx,
		`SELECT id FROM pipeline WHERE id IN ($1, $2)`, pl1, pl2)
	if err != nil {
		t.Fatalf("query none: %v", err)
	}
	defer rows.Close()
	seen = seen[:0]
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen = append(seen, id)
	}
	if len(seen) != 0 {
		t.Errorf("no project: visible pipelines = %v, want []", seen)
	}
	rows.Close()

	// Case 4: Bypass via DB() — raw *sql.DB still shows both rows because
	// the application user is the table owner (or has BYPASSRLS). The
	// default postgres role in dev typically does not bypass RLS, so this
	// may show 0 rows; we just assert it does not error.
	rows, err = db.QueryContext(ctx,
		`SELECT id FROM pipeline WHERE id IN ($1, $2)`, pl1, pl2)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}
	rows.Close()
}
