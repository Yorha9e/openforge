package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	authadapter "openforge/internal/auth/adapter"
	rbacmw "openforge/internal/auth/middleware"
	"openforge/internal/auth/service"
	"openforge/internal/pipeline/domain"
	"openforge/internal/shared/profile"
)

func RegisterRoutes(of *profile.OpenForge, jwtSvc *service.JWTService, cfg *profile.Config) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Auth
	authMw := AuthMiddleware(jwtSvc)
	mux.HandleFunc("POST /api/auth/login", handleLogin(jwtSvc, cfg))
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

	// Pipeline (auth + role)
	mux.HandleFunc("GET /api/pipelines/{id}", withRole("observer", handleGetPipeline(of)))
	mux.HandleFunc("POST /api/projects/{id}/pipelines", withRole("pm", handleCreatePipeline(of)))

	// Gate approval (pm + dev_lead)
	mux.HandleFunc("GET /api/review-inbox", withRoles([]string{"pm", "dev_lead"}, handleReviewInbox(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}", withRoles([]string{"pm", "dev_lead"}, handleApproveGate(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}/reject", withRoles([]string{"pm", "dev_lead"}, handleRejectGate(of)))

	// Pipeline fork (pm)
	mux.HandleFunc("POST /api/pipelines/{id}/fork", withRole("pm", handleForkPipeline(of)))

	// Token/Cost (pm)
	mux.HandleFunc("GET /api/projects/{id}/token-usage", withRole("pm", handleTokenUsage(of)))
	mux.HandleFunc("GET /api/projects/{id}/token-budget", withRole("pm", handleTokenBudget(of)))

	// Models (observer)
	mux.HandleFunc("GET /api/models", withRole("observer", handleListModels(of)))

	// Settings (auth)
	mux.HandleFunc("GET /api/user/settings", authMw(handleGetSettings()))
	mux.HandleFunc("PUT /api/user/settings", authMw(handleUpdateSettings()))

	// Admin status (admin)
	mux.HandleFunc("GET /api/admin/status", withAdmin(handleAdminStatus(of, cfg)))

	// Pipeline messages (observer)
	mux.HandleFunc("GET /api/pipelines/{pid}/messages", withRole("observer", handleGetMessages(of)))

	// OIDC (conditionally registered when auth.provider is "oidc")
	if cfg.Auth.Provider == "oidc" {
		oidcProvider := authadapter.NewOIDCProvider(cfg.Auth.OIDC)
		mux.HandleFunc("GET /api/auth/oidc/login", handleOIDCLogin(oidcProvider))
		mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(oidcProvider, jwtSvc))
	}

	// WebSocket (auth via first-frame protocol, not HTTP header)
	mux.HandleFunc("GET /ws/chat", handleChatWS(of, jwtSvc))

	// Static files
	mux.HandleFunc("GET /", handleStatic())

	return CorsMiddleware(SecurityHeadersMiddleware(LoggingMiddleware(mux)))
}

// sanitizeError returns a user-safe error message, logging the real error.
// Prevents leaking infrastructure details (DB hosts, ports, connection strings) to the frontend.
func sanitizeError(err error) string {
	msg := err.Error()
	// Database connection details
	if strings.Contains(msg, "dial tcp") || strings.Contains(msg, "connect") || strings.Contains(msg, "connection refused") {
		log.Printf("[ERROR] database unavailable: %v", err)
		return "service temporarily unavailable, please try again later"
	}
	// PostgreSQL driver errors
	if strings.Contains(msg, "pq:") {
		log.Printf("[ERROR] database error: %v", err)
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

func handleLogin(jwtSvc *service.JWTService, cfg *profile.Config) http.HandlerFunc {
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

		// Authenticate against builtin users (dev/prod-jwt mode)
		user, ok := cfg.Auth.Authenticate(req.Username, req.Password)
		if !ok {
			writeError(w, 401, "invalid credentials")
			return
		}

		token, err := jwtSvc.Issue(user.Username, user.Role, "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, map[string]any{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"display_name":  user.DisplayName,
			"role":          user.Role,
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

func handleListProjects(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := of.PipelineRepo.ListByProject(r.Context(), "")
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		if projects == nil {
			projects = []*domain.Pipeline{}
		}
		writeJSON(w, 200, projects)
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

type userSettings struct {
	Notifications map[string]bool `json:"notifications"`
	DefaultLayout string          `json:"default_layout"`
	Theme         string          `json:"theme"`
	FontSize      int             `json:"font_size"`
}

var defaultSettings = userSettings{
	Notifications: map[string]bool{"pipeline": true, "gate": true, "token": true, "weekly_report": false},
	DefaultLayout: "simple",
	Theme:         "dark",
	FontSize:      14,
}

// Session-scoped settings store (in-memory, lost on restart — Phase 7: persist to DB).
var settingsStore = map[string]*userSettings{}

func getSettingsStore(userID string) *userSettings {
	if s, ok := settingsStore[userID]; ok {
		return s
	}
	cp := defaultSettings
	cp.Notifications = make(map[string]bool)
	for k, v := range defaultSettings.Notifications {
		cp.Notifications[k] = v
	}
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
		if req.Notifications != nil {
			s.Notifications = req.Notifications
		}
		if req.DefaultLayout != "" {
			s.DefaultLayout = req.DefaultLayout
		}
		if req.Theme != "" {
			s.Theme = req.Theme
		}
		if req.FontSize > 0 {
			s.FontSize = req.FontSize
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
		writeJSON(w, 200, map[string]any{
			"phase":           "Phase 6.5",
			"profile":          cfg.Profile,
			"tier":             cfg.SecurityTier,
			"skills":           skillCount,
			"rbac":             "active",
			"oidc":             map[bool]string{true: "enabled", false: "disabled"}[cfg.Auth.Provider == "oidc"],
			"auth_provider":    cfg.Auth.Provider,
			"models":           len(of.LLMRouter.ListModels()),
		})
	}
}

// --- Pipeline messages ---

func handleGetMessages(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Phase 7: read from conversation_message table
		// Phase 6.5: return empty — messages are ephemeral (WS only)
		writeJSON(w, 200, []any{})
	}
}

func handleStatic() http.HandlerFunc {
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

func handleOIDCCallback(provider *authadapter.OIDCProvider, jwtSvc *service.JWTService) http.HandlerFunc {
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
		token, err := jwtSvc.Issue(user.Email, "pm", "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, token)
	}
}
