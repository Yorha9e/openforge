package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	authadapter "openforge/internal/auth/adapter"
	rbacmw "openforge/internal/auth/middleware"
	authport "openforge/internal/auth/port"
	"openforge/internal/auth/service"
	authdomain "openforge/internal/auth/domain"
	"openforge/internal/pipeline/domain"
	observabilitydomain "openforge/internal/observability/domain"
	port2 "openforge/internal/pipeline/port"
	"openforge/internal/shared/featureflags"
	"openforge/internal/shared/profile"
)

func RegisterRoutes(of *profile.OpenForge, jwtSvc *service.JWTService, cfg *profile.Config) http.Handler {
	mux := http.NewServeMux()

	// Rate limiting (100 req/s per IP)
	rateLimit := RateLimitMiddleware(100)

	// Multi-tenant project access isolation.
	authRepo := authadapter.NewPGAuthRepository(of.DB)
	tenantMw := TenantMiddleware(authRepo)

	// Background gate timeout scanner — prevents indefinite goroutine deadlock.
	StartGateTimeoutChecker(of.GateRequestRepo, of.PipelineSvc)

	// Background project/pipeline soft-delete hard-cleanup (30-day retention).
	StartSoftDeleteCleaner(of.DB)

	// Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Database health check with connection pool stats
	mux.HandleFunc("GET /api/health/db", func(w http.ResponseWriter, r *http.Request) {
		dbStats := of.DB.Stats()
		json.NewEncoder(w).Encode(map[string]any{
			"status":              "ok",
			"max_open_connections": dbStats.MaxOpenConnections,
			"open_connections":    dbStats.OpenConnections,
			"in_use":              dbStats.InUse,
			"idle":                dbStats.Idle,
			"wait_count":          dbStats.WaitCount,
			"wait_duration":       dbStats.WaitDuration.String(),
			"max_idle_closed":     dbStats.MaxIdleClosed,
			"max_lifetime_closed": dbStats.MaxLifetimeClosed,
		})
	})

	// Prometheus Metrics
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, of.PrometheusExporter.FormatMetrics())
	})

	// Auth
	authMw := AuthMiddleware(jwtSvc)
	mux.HandleFunc("POST /api/auth/login", handleLogin(jwtSvc, cfg, authRepo))
	mux.HandleFunc("POST /api/auth/register", handleRegister(jwtSvc, authRepo))
	mux.HandleFunc("POST /api/auth/refresh", handleRefresh(jwtSvc))

	// RBAC helpers
	withRole := func(role string, next http.HandlerFunc) http.HandlerFunc {
		return authMw(rbacmw.RequireRoleMiddleware(role, next))
	}
	withRoles := func(roles []string, next http.HandlerFunc) http.HandlerFunc {
		return authMw(rbacmw.RequireRolesMiddleware(roles, next))
	}

	// Admin-only wrapper
	withAdmin := func(next http.HandlerFunc) http.HandlerFunc {
		return authMw(rbacmw.RequireRoleMiddleware("admin", next))
	}

	// Phase 6.5: Skill endpoints (admin or auth required)
	RegisterSkillRoutes(mux, of, withAdmin, authMw)

	// Projects (auth + role)
	mux.HandleFunc("GET /api/projects", withRole("observer", handleListProjects(of)))
	mux.HandleFunc("GET /api/projects/{id}", withRole("observer", handleGetProject(of)))
	mux.HandleFunc("POST /api/projects", withRole("pm", handleCreateProject(of)))
	mux.HandleFunc("DELETE /api/projects/{id}", withRole("pm", handleDeleteProject(of)))

	// Pipeline (auth + role)
	mux.HandleFunc("GET /api/pipelines/{id}", withRole("observer", handleGetPipeline(of)))
	mux.HandleFunc("GET /api/pipelines/{id}/diff", withRole("observer", handleGetDiff(of)))
	mux.HandleFunc("GET /api/projects/{id}/pipelines", withRole("observer", handleListPipelines(of)))
	mux.HandleFunc("POST /api/projects/{id}/pipelines", withRole("pm", handleCreatePipeline(of)))
	
	// Active pipelines (observer) — cross-project active workboard
	mux.HandleFunc("GET /api/pipelines/active", withRole("observer", handleActivePipelines(of, authRepo)))

	// Gate approval (pm + dev_lead)
	mux.HandleFunc("GET /api/review-inbox", withRoles([]string{"pm", "dev_lead"}, handleReviewInbox(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}", withRoles([]string{"pm", "dev_lead"}, handleApproveGate(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}/reject", withRoles([]string{"pm", "dev_lead"}, handleRejectGate(of)))

	// Pipeline fork (pm)
	mux.HandleFunc("POST /api/pipelines/{id}/fork", withRole("pm", handleForkPipeline(of)))

	// Pipeline delete (pm)
	mux.HandleFunc("DELETE /api/pipelines/{id}", withRole("pm", handleDeletePipeline(of)))

	// Token/Cost (pm)
	mux.HandleFunc("GET /api/projects/{id}/token-usage", withRole("pm", handleTokenUsage(of)))
	mux.HandleFunc("GET /api/projects/{id}/token-budget", withRole("pm", handleTokenBudget(of)))

	// Models (observer)
	mux.HandleFunc("GET /api/models", withRole("observer", handleListModels(of)))

	// Settings (auth)
	mux.HandleFunc("GET /api/settings", authMw(handleGetSettings()))
	mux.HandleFunc("PUT /api/settings", authMw(handleUpdateSettings()))

	// Admin status (admin)
	mux.HandleFunc("GET /api/admin/status", withAdmin(handleAdminStatus(of, cfg)))

	// Admin: Feature flags (admin-only, runtime toggles without restart)
	ffStore := featureflags.NewStore(of.DB)
	mux.HandleFunc("GET /api/admin/feature-flags", withAdmin(handleGetFeatureFlags(of)))
	mux.HandleFunc("PUT /api/admin/feature-flags", withAdmin(handleUpdateFeatureFlags(of, ffStore)))

	// Admin: Experiments (admin-only)
	mux.HandleFunc("GET /api/admin/experiments", withAdmin(handleListExperiments()))

	// Compliance suite: Audit log export (admin-only)
	mux.HandleFunc("GET /api/admin/audit/export", withAdmin(handleAuditExport(of)))

	// Production ops: Runbook API (auth required)
	mux.HandleFunc("GET /api/runbook", authMw(handleRunbookList(of)))
	mux.HandleFunc("GET /api/runbook/{id}", authMw(handleRunbookDetail(of)))

	// Distribution artifacts: Offline deployment bundle download (admin-only)
	mux.HandleFunc("GET /api/download/offline", withAdmin(handleDownloadOffline(of)))

	// Pipeline messages (observer)
	mux.HandleFunc("GET /api/pipelines/{pid}/messages", withRole("observer", handleGetMessages(of)))

	// Pipeline branches (observer)
	mux.HandleFunc("GET /api/pipelines/{pid}/branches", withRole("observer", handleListBranches(of)))

	// File system browsing (observer)
	mux.HandleFunc("GET /api/files", withRole("observer", handleListFiles()))
	mux.HandleFunc("GET /api/files/content", withRole("observer", handleFileContent()))

	// OIDC (conditionally registered when auth.provider is "oidc")
	if cfg.Auth.Provider == "oidc" {
		oidcProvider := authadapter.NewOIDCProvider(cfg.Auth.OIDC)
		mux.HandleFunc("GET /api/auth/oidc/login", handleOIDCLogin(oidcProvider))
		mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(oidcProvider, jwtSvc, authRepo))
	}

	// WebSocket (auth via first-frame protocol, not HTTP header)
	mux.HandleFunc("GET /ws/chat", handleChatWS(of, jwtSvc))

	// Static files
	mux.HandleFunc("GET /", handleStatic())

	// Load-shedding middleware: only active when SLO tracker is available.
	// Default: permissive (accepts all priorities when capacity is normal).
	var handler http.Handler = CorsMiddleware(SecurityHeadersMiddleware(LoggingMiddleware(rateLimit(tenantMw(mux)))))
	if of.SLO != nil {
		ls := observabilitydomain.NewLoadShedder()
		provider := &ofResourceSnapshotProvider{of: of}
		handler = LoadShedMiddleware(ls, provider, handler)
	}
	return handler
}

// sanitizeError returns a user-safe error message, logging the real error.
// Prevents leaking infrastructure details (DB hosts, ports, connection strings) to the frontend.
func sanitizeError(err error) string {
	msg := err.Error()
	// Database connection details
	if strings.Contains(msg, "dial tcp") || strings.Contains(msg, "connect") || strings.Contains(msg, "connection refused") {
		slog.Error("database unavailable", "error", err)
		return "service temporarily unavailable, please try again later"
	}
	// PostgreSQL driver errors
	if strings.Contains(msg, "pq:") {
		slog.Error("database error", "error", err)
		return "request failed, please try again"
	}
	return msg
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleLogin(jwtSvc *service.JWTService, cfg *profile.Config, authRepo authport.AuthRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}
		if req.Username == "" || req.Password == "" {
			writeError(w, 400, "username and password required")
			return
		}

		var userID, displayName, role string

		// 1. Try builtin users from config
		if user, ok := cfg.Auth.Authenticate(req.Username, req.Password); ok {
			userID = user.Username
			displayName = user.DisplayName
			role = user.Role

			// Ensure config-based user exists in "user" table for FK satisfaction.
			// This is idempotent (ON CONFLICT DO NOTHING in repo implementation).
			_ = authRepo.CreateUser(r.Context(), &authport.User{
				ID:          userID,
				DisplayName: displayName,
			})
		} else {
			// 2. Try DB-backed registered users
			hash, err := authRepo.GetUserPasswordHash(r.Context(), req.Username)
			if err != nil || hash == "" || !profile.CheckPassword(req.Password, hash) {
				writeError(w, 401, "invalid credentials")
				return
			}
			// Resolve user details from DB
			u, err := authRepo.GetUser(r.Context(), req.Username)
			if err != nil || u == nil {
				writeError(w, 401, "invalid credentials")
				return
			}
			userID = u.ID
			displayName = u.DisplayName
			// Registered users default to 'pm' role if no explicit global role is set
			role = "pm"
		}

		token, err := jwtSvc.Issue(userID, role, "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, map[string]any{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"display_name":  displayName,
			"role":          role,
		})
	}
}

func handleRefresh(jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}
		claims, err := jwtSvc.Verify(req.RefreshToken)
		if err != nil {
			writeError(w, 401, "invalid refresh token")
			return
		}
		token, err := jwtSvc.Issue(claims.UserID, claims.Role, claims.ProjectID)
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, token)
	}
}

func handleRegister(jwtSvc *service.JWTService, authRepo authport.AuthRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username    string `json:"username"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
			AvatarURL   string `json:"avatar_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}
		if req.Username == "" || req.Password == "" {
			writeError(w, 400, "username and password required")
			return
		}
		if req.DisplayName == "" {
			req.DisplayName = req.Username
		}

		// Check if user already exists
		existing, _ := authRepo.GetUser(r.Context(), req.Username)
		if existing != nil {
			writeError(w, 409, "username already exists")
			return
		}

		// Hash password
		hash, err := profile.HashPassword(req.Password)
		if err != nil {
			writeError(w, 500, "failed to process password")
			return
		}

		// Create user with password hash
		if err := authRepo.RegisterUser(r.Context(), req.Username, req.DisplayName, hash); err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
				writeError(w, 409, "username already exists")
				return
			}
			writeError(w, 500, sanitizeError(err))
			return
		}

		// Auto-login: issue JWT
		token, err := jwtSvc.Issue(req.Username, "pm", "")
		if err != nil {
			writeError(w, 500, "registration succeeded but failed to issue token")
			return
		}
		writeJSON(w, 201, map[string]any{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"display_name":  req.DisplayName,
			"role":          "pm",
		})
	}
}

func handleListProjects(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		userRole := UserRoleFromContext(r.Context())

		// Admin users see all projects
		var query string
		var args []interface{}
		if userRole == "admin" {
			query = `SELECT id, name, git_url, created_at FROM project WHERE deleted_at IS NULL ORDER BY created_at DESC`
		} else {
			// Regular users only see projects they have a role in
			query = `SELECT p.id, p.name, p.git_url, p.created_at
				FROM project p
				INNER JOIN user_role ur ON p.id = ur.project_id
				WHERE ur.user_id = $1 AND p.deleted_at IS NULL
				ORDER BY p.created_at DESC`
			args = append(args, userID)
		}

		rows, err := of.DB.QueryContext(r.Context(), query, args...)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		defer rows.Close()
		type project struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			GitURL    string `json:"git_url"`
			CreatedAt string `json:"created_at"`
		}
		var projects []project
		for rows.Next() {
			var p project
			if err := rows.Scan(&p.ID, &p.Name, &p.GitURL, &p.CreatedAt); err != nil {
				writeError(w, 500, sanitizeError(err))
				return
			}
			projects = append(projects, p)
		}
		if projects == nil {
			projects = []project{}
		}
		writeJSON(w, 200, projects)
	}
}

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

func handleGetProject(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		userID := UserIDFromContext(r.Context())
		userRole := UserRoleFromContext(r.Context())

		// Non-admin users: verify project access via user_role
		if userRole != "admin" {
			var exists bool
			err := of.DB.QueryRowContext(r.Context(),
				`SELECT EXISTS(SELECT 1 FROM user_role WHERE user_id = $1 AND project_id = $2)`,
				userID, id,
			).Scan(&exists)
			if err != nil || !exists {
				writeError(w, 403, "forbidden: access to this project is denied")
				return
			}
		}

		var p struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			GitURL    string `json:"git_url"`
			CreatedAt string `json:"created_at"`
		}
		err := of.DB.QueryRowContext(r.Context(),
			`SELECT id, name, git_url, created_at FROM project WHERE id = $1 AND deleted_at IS NULL`, id).
			Scan(&p.ID, &p.Name, &p.GitURL, &p.CreatedAt)
		if err != nil {
			writeError(w, 404, "project not found")
			return
		}
		writeJSON(w, 200, p)
	}
}

func handleGetPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p, err := of.PipelineRepo.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, 404, err.Error())
			return
		}
		writeJSON(w, 200, p)
	}
}

func handleListPipelines(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		pipelines, err := of.PipelineRepo.ListByProject(r.Context(), projectID)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, pipelines)
	}
}

func handleActivePipelines(of *profile.OpenForge, authRepo authport.AuthRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		userRole := UserRoleFromContext(r.Context())

		var rows *sql.Rows
		var err error
		if userRole == "admin" {
			rows, err = of.DB.QueryContext(r.Context(),
				`SELECT pl.id, pl.project_id, pr.name as project_name, pl.title, pl.status, pl.current_stage, pl.updated_at
				FROM pipeline pl
				INNER JOIN project pr ON pl.project_id = pr.id
				WHERE pl.status IN ('running','paused','awaiting_review')
				AND pl.deleted_at IS NULL
				ORDER BY pl.updated_at DESC`)
		} else {
			rows, err = of.DB.QueryContext(r.Context(),
				`SELECT pl.id, pl.project_id, pr.name as project_name, pl.title, pl.status, pl.current_stage, pl.updated_at
				FROM pipeline pl
				INNER JOIN project pr ON pl.project_id = pr.id
				INNER JOIN user_role ur ON pr.id = ur.project_id
				WHERE ur.user_id = $1
				AND pl.status IN ('running','paused','awaiting_review')
				AND pl.deleted_at IS NULL
				ORDER BY pl.updated_at DESC`, userID)
		}
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		defer rows.Close()

		type activePipeline struct {
			ID           string `json:"id"`
			ProjectID    string `json:"project_id"`
			ProjectName  string `json:"project_name"`
			Title        string `json:"title"`
			Status       string `json:"status"`
			CurrentStage string `json:"current_stage"`
			UpdatedAt    string `json:"updated_at"`
		}
		var result []activePipeline
		for rows.Next() {
			var ap activePipeline
			if err := rows.Scan(&ap.ID, &ap.ProjectID, &ap.ProjectName, &ap.Title, &ap.Status, &ap.CurrentStage, &ap.UpdatedAt); err != nil {
				writeError(w, 500, sanitizeError(err))
				return
			}
			result = append(result, ap)
		}
		if result == nil {
			result = []activePipeline{}
		}
		writeJSON(w, 200, result)
	}
}

func handleCreatePipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		projectID := r.PathValue("id")
		userID := UserIDFromContext(r.Context())
		p := domain.NewPipeline(
			"pipe-"+fmt.Sprintf("%d", time.Now().UnixNano()),
			projectID, req.Title, userID, 1, 1,
		)
		if err := of.PipelineRepo.Create(r.Context(), p); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 201, p)
	}
}

func handleApproveGate(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("id")
		stage := r.PathValue("stage")
		actor := UserIDFromContext(r.Context())

		var req struct {
			Checklist domain.GateChecklist `json:"checklist"`
			Summary   string               `json:"summary_feedback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := of.GateSvc.Approve(r.Context(), pipelineID, stage, actor, req.Checklist, req.Summary); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, map[string]string{"status": "approved"})
	}
}

func handleRejectGate(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("id")
		stage := r.PathValue("stage")
		actor := UserIDFromContext(r.Context())

		var req struct {
			Comments []domain.LineComment `json:"line_comments"`
			Summary  string               `json:"summary_feedback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := of.GateSvc.Reject(r.Context(), pipelineID, stage, actor, req.Comments, req.Summary); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, map[string]string{"status": "rejected"})
	}
}

func handleReviewInbox(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := of.GateSvc.ListPending(r.Context())
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		if events == nil {
				events = []*domain.GateEvent{}
			}
			writeJSON(w, 200, events)
	}
}

// --- Token/Cost endpoints ---

func handleTokenUsage(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		days := 30
		if d := r.URL.Query().Get("days"); d != "" {
			if _, err := fmt.Sscanf(d, "%d", &days); err != nil {
				writeError(w, http.StatusBadRequest, "invalid days parameter")
				return
			}
		}
		rows, err := of.TokenCostSvc.DailyUsage(r.Context(), projectID, days)
		if err != nil {
			writeError(w, http.StatusInternalServerError, sanitizeError(err))
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func handleTokenBudget(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		b, err := of.TokenCostSvc.Budget(r.Context(), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, sanitizeError(err))
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}

func handleListModels(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		models := of.LLMRouter.ListModels()
		writeJSON(w, http.StatusOK, models)
	}
}

func handleDeletePipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// 1. 获取 Pipeline 详情
		p, err := of.PipelineRepo.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 越权校验
		ctxProjectID, _ := r.Context().Value(authdomain.ProjectIDContextKey).(string)
		if ctxProjectID != "" && p.ProjectID != ctxProjectID {
			writeError(w, http.StatusForbidden, "forbidden: deletion of this pipeline is denied")
			return
		}

		// 3. 执行软删除
		if err := of.PipelineRepo.Delete(r.Context(), id); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	}
}

func handleForkPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Title string `json:"title"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		child, err := of.PipelineSvc.Fork(r.Context(), r.PathValue("id"), req.Title, UserIDFromContext(r.Context()))
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 201, child)
	}
}

// --- Settings (in-memory, per-session) ---

type userNotificationSettings struct {
	EmailEnabled bool     `json:"emailEnabled"`
	WebhookURL   string   `json:"webhookUrl"`
	Channels     []string `json:"channels"`
}

type userLayoutSettings struct {
	EditorFontSize int    `json:"editorFontSize"`
	Theme          string `json:"theme"`
	DefaultViewMode string `json:"defaultViewMode"`
}

type userLanguageSettings struct {
	Locale   string `json:"locale"`
	Timezone string `json:"timezone"`
}

type userProjectSettings struct {
	WorkDir string `json:"workDir"`
}

type userSettings struct {
	Notifications userNotificationSettings `json:"notifications"`
	Layout        userLayoutSettings       `json:"layout"`
	Language      userLanguageSettings     `json:"language"`
	Project       userProjectSettings      `json:"project"`
}

var defaultSettings = userSettings{
	Notifications: userNotificationSettings{
		EmailEnabled: true,
		WebhookURL:   "",
		Channels:     []string{"email"},
	},
	Layout: userLayoutSettings{
		EditorFontSize: 14,
		Theme:          "dark",
		DefaultViewMode: "pro",
	},
	Language: userLanguageSettings{
		Locale:   "en",
		Timezone: "UTC",
	},
	Project: userProjectSettings{
		WorkDir: "",
	},
}

// Session-scoped settings store (in-memory, lost on restart — Phase 7: persist to DB).
var settingsStore = map[string]*userSettings{}

func getSettingsStore(userID string) *userSettings {
	if s, ok := settingsStore[userID]; ok {
		return s
	}
	cp := defaultSettings
	cp.Notifications.Channels = make([]string, len(defaultSettings.Notifications.Channels))
	copy(cp.Notifications.Channels, defaultSettings.Notifications.Channels)
	settingsStore[userID] = &cp
	return &cp
}

func handleGetSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSettingsStore(UserIDFromContext(r.Context()))
		writeJSON(w, 200, s)
	}
}

func handleUpdateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req userSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		s := getSettingsStore(UserIDFromContext(r.Context()))
		
		// Update notifications if provided
		if req.Notifications.EmailEnabled || req.Notifications.WebhookURL != "" || len(req.Notifications.Channels) > 0 {
			s.Notifications = req.Notifications
		}
		
		// Update layout if provided
		if req.Layout.EditorFontSize > 0 {
			s.Layout.EditorFontSize = req.Layout.EditorFontSize
		}
		if req.Layout.Theme != "" {
			s.Layout.Theme = req.Layout.Theme
		}
		if req.Layout.DefaultViewMode != "" {
			s.Layout.DefaultViewMode = req.Layout.DefaultViewMode
		}
		
		// Update language if provided
		if req.Language.Locale != "" {
			s.Language.Locale = req.Language.Locale
		}
		if req.Language.Timezone != "" {
			s.Language.Timezone = req.Language.Timezone
		}
		
		// Update project settings if provided
		if req.Project.WorkDir != "" {
			s.Project.WorkDir = req.Project.WorkDir
		}
		
		writeJSON(w, 200, s)
	}
}

// --- Admin status ---

func handleAdminStatus(of *profile.OpenForge, cfg *profile.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		skillCount := 0
		if of.SkillLoader != nil {
			skillCount = len(of.SkillLoader.GetAllSkills())
		}
		
		breakers := make(map[string]string)
		if of.BreakerPool != nil {
			for name, state := range of.BreakerPool.All() {
				breakers[name] = state.String()
			}
		}

		var sloObj map[string]any
		if of.SLO != nil {
			snap := of.SLO.Snapshot()
			sloObj = map[string]any{
				"total":        snap.Total,
				"success_rate": snap.SuccessRate,
				"p95_ms":       snap.P95Ms,
			}
		}

		writeJSON(w, 200, map[string]any{
			"phase":            "Phase 8",
			"profile":          cfg.Profile,
			"tier":             cfg.SecurityTier,
			"skills":           skillCount,
			"rbac":             "active",
			"oidc":             map[bool]string{true: "enabled", false: "disabled"}[cfg.Auth.Provider == "oidc"],
			"auth_provider":    cfg.Auth.Provider,
			"models":           len(of.LLMRouter.ListModels()),
			"circuit_breakers": breakers,
			"slo":              sloObj,
			"ha": map[string]any{
				"task_queue":       cfg.TaskQueue,
				"hash_ring_nodes":  128,
				"load_shedding":    "active",
			},
		})
	}
}

// --- Pipeline messages ---

func handleGetMessages(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("pid")
		branchID := r.URL.Query().Get("branch_id")
		if branchID == "" {
			branchID = "main"
		}

		// 1. 获取 Pipeline 详情，以便验证归属项目
		p, err := of.PipelineRepo.GetByID(r.Context(), pipelineID)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 越权校验: 如果上下文有关联 ProjectID，进行一致性校验
		ctxProjectID, _ := r.Context().Value(authdomain.ProjectIDContextKey).(string)
		if ctxProjectID != "" && p.ProjectID != ctxProjectID {
			writeError(w, http.StatusForbidden, "forbidden: access to this pipeline is denied")
			return
		}

		msgs, err := of.PipelineRepo.GetMessages(r.Context(), pipelineID, branchID)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		if msgs == nil {
			msgs = []*port2.DBMessage{}
		}
		writeJSON(w, 200, msgs)
	}
}

func handleListBranches(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("pid")

		// 1. 获取 Pipeline 详情，以便验证归属项目
		p, err := of.PipelineRepo.GetByID(r.Context(), pipelineID)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 越权校验: 如果上下文有关联 ProjectID，进行一致性校验
		ctxProjectID, _ := r.Context().Value(authdomain.ProjectIDContextKey).(string)
		if ctxProjectID != "" && p.ProjectID != ctxProjectID {
			writeError(w, http.StatusForbidden, "forbidden: access to this pipeline is denied")
			return
		}

		branches, err := of.PipelineRepo.ListBranches(r.Context(), pipelineID)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, map[string]any{"branches": branches})
	}
}

func handleListExperiments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement experiments feature
		// For now, return empty array
		writeJSON(w, 200, []any{})
	}
}

func handleListFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dirPath := r.URL.Query().Get("path")
		if dirPath == "" {
			writeError(w, 400, "path parameter required")
			return
		}

		// Security: prevent directory traversal
		cleanPath := filepath.Clean(dirPath)
		if strings.Contains(cleanPath, "..") {
			writeError(w, 400, "invalid path")
			return
		}

		entries, err := os.ReadDir(cleanPath)
		if err != nil {
			writeError(w, 400, fmt.Sprintf("cannot read directory: %v", err))
			return
		}

		type FileInfo struct {
			Name  string `json:"name"`
			IsDir bool   `json:"is_dir"`
			Size  int64  `json:"size"`
			Path  string `json:"path"`
		}

		files := make([]FileInfo, 0, len(entries))
		for _, entry := range entries {
			// Skip hidden files
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, FileInfo{
				Name:  entry.Name(),
				IsDir: entry.IsDir(),
				Size:  info.Size(),
				Path:  filepath.Join(cleanPath, entry.Name()),
			})
		}

		writeJSON(w, 200, map[string]any{
			"files": files,
			"count": len(files),
			"path":  cleanPath,
		})
	}
}

func handleFileContent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			writeError(w, 400, "path parameter required")
			return
		}

		// Security: prevent directory traversal
		cleanPath := filepath.Clean(filePath)
		if strings.Contains(cleanPath, "..") {
			writeError(w, 400, "invalid path")
			return
		}

		// Check if file exists and is a regular file
		info, err := os.Stat(cleanPath)
		if err != nil {
			writeError(w, 404, fmt.Sprintf("file not found: %v", err))
			return
		}
		if info.IsDir() {
			writeError(w, 400, "path is a directory, not a file")
			return
		}

		// Read file content
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			writeError(w, 500, fmt.Sprintf("cannot read file: %v", err))
			return
		}

		// Determine language from file extension
		ext := strings.ToLower(filepath.Ext(cleanPath))
		langMap := map[string]string{
			".ts": "typescript", ".tsx": "typescript",
			".js": "javascript", ".jsx": "javascript",
			".go": "go", ".py": "python", ".rs": "rust",
			".java": "java", ".json": "json",
			".yaml": "yaml", ".yml": "yaml",
			".md": "markdown", ".sql": "sql",
			".html": "html", ".css": "css", ".scss": "scss",
		}
		language := langMap[ext]
		if language == "" {
			language = "plaintext"
		}

		writeJSON(w, 200, map[string]any{
			"content":  string(content),
			"language": language,
			"path":     cleanPath,
			"size":     info.Size(),
		})
	}
}

func handleStatic() http.HandlerFunc {
	distDir := "frontend/dist"
	if info, err := os.Stat(distDir); err != nil || !info.IsDir() {
		return devStaticHandler()
	}
	return buildStaticHandler(distDir)
}

func buildStaticHandler(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		// SPA fallback: if the file doesn't exist, serve index.html
		fullPath := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	}
}

func devStaticHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Static-File", "not-implemented")
		writeError(w, 404, "static files not available in dev mode (use Vite dev server)")
	}
}

// --- OIDC handlers ---

func handleOIDCLogin(provider *authadapter.OIDCProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if state == "" {
			state = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		url, err := provider.AuthCodeURL(state)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"url": url, "state": state})
	}
}

func handleOIDCCallback(provider *authadapter.OIDCProvider, jwtSvc *service.JWTService, authRepo authport.AuthRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			writeError(w, 400, "missing code parameter")
			return
		}
		user, err := provider.Exchange(r.Context(), code)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		// Ensure OIDC user exists in "user" table for FK satisfaction.
		_ = authRepo.CreateUser(r.Context(), &authport.User{
			ID:          user.Email,
			DisplayName: user.Name,
		})

		token, err := jwtSvc.Issue(user.Email, "pm", "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, token)
	}
}

func handleDeleteProject(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		userID := UserIDFromContext(r.Context())
		userRole := UserRoleFromContext(r.Context())

		// 1. Verify project exists and is not already deleted
		var projectName string
		err := of.DB.QueryRowContext(r.Context(),
			`SELECT name FROM project WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&projectName)
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}

		// 2. Permission check: admin (global) or pm+ within this project
		if userRole != "admin" {
			var projectRole string
			err := of.DB.QueryRowContext(r.Context(),
				`SELECT role FROM user_role WHERE user_id = $1 AND project_id = $2`,
				userID, id,
			).Scan(&projectRole)
			if err != nil || (projectRole != "admin" && projectRole != "pm") {
				writeError(w, http.StatusForbidden, "forbidden: deletion requires admin or pm role on this project")
				return
			}
		}

		// 3. Execute cascading soft delete in a transaction
		tx, err := of.DB.BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		defer tx.Rollback()

		// Soft-delete all pipelines in this project
		_, err = tx.ExecContext(r.Context(),
			`UPDATE pipeline SET deleted_at = NOW(), updated_at = NOW()
			 WHERE project_id = $1 AND deleted_at IS NULL`, id)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		// Soft-delete the project itself
		res, err := tx.ExecContext(r.Context(),
			`UPDATE project SET deleted_at = NOW()
			 WHERE id = $1 AND deleted_at IS NULL`, id)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}

		slog.Info("project soft-deleted",
			"project_id", id,
			"project_name", projectName,
			"deleted_by", userID,
		)

		writeJSON(w, 200, map[string]string{
			"status":  "deleted",
			"message": "Project soft-deleted. Data recoverable for 30 days.",
		})
	}
}

func handleGetDiff(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("id")
		filePath := r.URL.Query().Get("file")

		// 1. 获取 Pipeline 详情
		p, err := of.PipelineRepo.GetByID(r.Context(), pipelineID)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 如果指定了文件，查找对应的 ChangedFile
		if filePath != "" {
			for _, cf := range p.ChangedFiles {
				if cf.Path == filePath {
					writeJSON(w, 200, cf)
					return
				}
			}
			writeError(w, http.StatusNotFound, "file not found in pipeline changes")
			return
		}

		// 3. 如果没有指定文件，返回所有变更文件
		if p.ChangedFiles == nil {
			p.ChangedFiles = []domain.ChangedFile{}
		}
		writeJSON(w, 200, p.ChangedFiles)
	}
}
