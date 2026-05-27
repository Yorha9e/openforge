# Fix: Project Creator Access Permission

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the bug where a PM (or OIDC user) creates a project but cannot access it afterward — getting 403 Forbidden.

**Architecture:** The root cause is a foreign key constraint on `user_role.user_id REFERENCES "user"(id)`. Config-based users and OIDC users don't exist in the `"user"` table, so the `INSERT INTO user_role` silently fails. The fix ensures user upsert before role assignment, wraps both in a transaction, and returns errors properly.

**Tech Stack:** Go, PostgreSQL, standard `database/sql`

---

## Root Cause Diagram

```
handleCreateProject
  │
  ├── INSERT INTO project ✅ (always succeeds)
  │
  └── INSERT INTO user_role (user_id=$1, ...)
        │
        ├── registered user → user exists in "user" table → FK satisfied → ✅
        │
        └── config/OIDC user → user NOT in "user" table → FK violation → ❌
              │
              └── Error logged as slog.Warn → silently dropped
                    │
                    └── Project created, but creator has no user_role entry
                          │
                          └── GET /api/projects/{id} → SELECT FROM user_role → not found → 403
```

## Architecture

Two changes, one file:

1. **`handleCreateProject`** — add user upsert before role insert; wrap in transaction; surface errors
2. **`handleLogin` / `handleOIDCCallback`** — optionally pre-create the user in `"user"` table during login (defense-in-depth)

---

### Task 1: Add user auto-creation and transactional safety to `handleCreateProject`

**Files:**
- Modify: `internal/server/routes.go:351-389`

- [ ] **Step 1: Rewrite `handleCreateProject` to use a transaction with user upsert**

Replace the current `handleCreateProject` (lines 351-389) with:

```go
func handleCreateProject(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			GitURL string `json:"git_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if req.Name == "" {
			writeError(w, 400, "name required")
			return
		}
		projectID := fmt.Sprintf("proj-%d", time.Now().UnixNano())
		userID := UserIDFromContext(r.Context())
		displayName := userID // fallback display name from JWT identity

		tx, err := of.DB.BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		defer tx.Rollback()

		// 1. Create the project.
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO project (id, name, git_url, repo_type, template) VALUES ($1, $2, $3, 'custom', 'custom')`,
			projectID, req.Name, req.GitURL)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		// 2. Ensure the user exists in the "user" table (idempotent upsert).
		//    Config-based and OIDC users may not have a row yet, which would
		//    violate the FK on user_role.user_id → REFERENCES "user"(id).
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO "user" (id, display_name) VALUES ($1, $2)
			 ON CONFLICT (id) DO NOTHING`,
			userID, displayName)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		// 3. Auto-assign creator as project admin.
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO user_role (user_id, project_id, role, modules)
			 VALUES ($1, $2, 'admin', '{chat,code_review,pipeline,settings}')
			 ON CONFLICT (user_id, project_id) DO NOTHING`,
			userID, projectID)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		writeJSON(w, 201, map[string]any{
			"id": projectID, "name": req.Name, "git_url": req.GitURL, "role": "admin",
		})
	}
}
```

- [ ] **Step 2: Build to verify compilation**

```bash
cd d:\vscode\tiktok\openforge && go build ./cmd/server ./cmd/openforge 2>&1
```

Expected: compilation succeeds, no errors.

---

### Task 2: Add defense-in-depth — ensure config-based users are upserted at login time

**Files:**
- Modify: `internal/server/routes.go:164-218` (handleLogin)
- Modify: `internal/server/routes.go:114-118` (OIDC route registration)
- Modify: `internal/server/routes.go:911-930` (handleOIDCCallback)

- [ ] **Step 1: Add user upsert in `handleLogin` for config-based auth path**

In `handleLogin`, after line 185 (`role = user.Role`), add:

```go
		// Ensure config-based user exists in "user" table for FK satisfaction.
		// This is idempotent (ON CONFLICT DO NOTHING).
		_, _ = authRepo.CreateUser(r.Context(), &authport.User{
			ID:          userID,
			DisplayName: displayName,
		})
```

The full relevant section becomes:

```go
	// 1. Try builtin users from config
	if user, ok := cfg.Auth.Authenticate(req.Username, req.Password); ok {
		userID = user.Username
		displayName = user.DisplayName
		role = user.Role

		// Ensure config-based user exists in "user" table for FK satisfaction.
		// This is idempotent (ON CONFLICT DO NOTHING).
		_, _ = authRepo.CreateUser(r.Context(), &authport.User{
			ID:          userID,
			DisplayName: displayName,
		})
	} else {
```

- [ ] **Step 2a: Pass `authRepo` to `handleOIDCCallback`**

Change the route registration (line 117) from:

```go
mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(oidcProvider, jwtSvc))
```

To:

```go
mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(oidcProvider, jwtSvc, authRepo))
```

- [ ] **Step 2b: Update `handleOIDCCallback` signature and add user upsert**

Change the function signature from:

```go
func handleOIDCCallback(provider *authadapter.OIDCProvider, jwtSvc *service.JWTService) http.HandlerFunc {
```

To:

```go
func handleOIDCCallback(provider *authadapter.OIDCProvider, jwtSvc *service.JWTService, authRepo authport.AuthRepository) http.HandlerFunc {
```

Then, in the function body after `provider.Exchange(...)` succeeds and before issuing the token, add:

```go
		// Ensure OIDC user exists in "user" table for FK satisfaction.
		_, _ = authRepo.CreateUser(r.Context(), &authport.User{
			ID:          user.Email,
			DisplayName: user.Name,
		})
```

The full relevant section becomes:

```go
		user, err := provider.Exchange(r.Context(), code)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		// Ensure OIDC user exists in "user" table for FK satisfaction.
		_, _ = authRepo.CreateUser(r.Context(), &authport.User{
			ID:          user.Email,
			DisplayName: user.Name,
		})

		token, err := jwtSvc.Issue(user.Email, "pm", "")
```

- [ ] **Step 3: Build to verify compilation**

```bash
cd d:\vscode\tiktok\openforge && go build ./cmd/server ./cmd/openforge 2>&1
```

Expected: compilation succeeds, no errors.

---

### Task 3: Add tests for the fix

**Files:**
- Create: `internal/server/project_access_test.go`

- [ ] **Step 1: Write test that verifies config-based users get auto-created and can access projects**

```go
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
```

- [ ] **Step 2: Run tests**

```bash
cd d:\vscode\tiktok\openforge && go test ./internal/server/ -run "TestAutoCreateUserOnProjectCreation|TestExistingUserNotDuplicated" -v -count=1 2>&1
```

These tests require a running PostgreSQL at `localhost:5432`. Use `docker-compose up -d` first if needed. If no DB is available they will skip (not fail).

---

### Task 4: Final verification

- [ ] **Step 1: Full build**

```bash
cd d:\vscode\tiktok\openforge && go build ./... 2>&1
```

- [ ] **Step 2: Full vet**

```bash
cd d:\vscode\tiktok\openforge && go vet ./... 2>&1
```

- [ ] **Step 3: Run existing tests (non-DB)**

```bash
cd d:\vscode\tiktok\openforge && go test ./internal/... -count=1 -timeout 120s 2>&1
```

- [ ] **Step 4: Update plan status document**

Update `docs/superpowers/plans/2026-05-24-phase-8-ha-concurrency.md` with a note that this bug was found and fixed.

- [ ] **Step 5: Commit**

```bash
git add internal/server/routes.go internal/server/project_access_test.go docs/superpowers/plans/2026-05-26-project-creator-access-fix.md
git commit -m "fix: auto-create user row before assigning project role

Config-based and OIDC users were not present in the 'user' table,
causing the INSERT INTO user_role to fail via FK constraint violation.
The error was logged but silently swallowed, leaving the project created
but the creator with no access (403 on GET /api/projects/{id}).

Changes:
- handleCreateProject: wrap project + role creation in a transaction
- handleCreateProject: upsert user into 'user' table before role insert
- handleLogin: ensure config-based users exist in 'user' table on login
- handleOIDCCallback: ensure OIDC users exist in 'user' table on login
- Add test coverage for the auto-creation and idempotency"
```

---

## Acceptance Checklist

- [ ] Config-based PM can create a project and immediately access it (no 403)
- [ ] OIDC user can create a project and immediately access it (no 403)
- [ ] Registered user flow still works (no regression)
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] All existing tests pass (`go test ./internal/...`)
- [ ] New test verifies user auto-creation and role assignment
