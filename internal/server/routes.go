package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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

	// RBAC helper — wraps authMw with role requirement
	withRole := func(role string, next http.HandlerFunc) http.HandlerFunc {
		return authMw(rbacmw.RequireRoleMiddleware(role, next))
	}

	// Projects (auth + role)
	mux.HandleFunc("GET /api/projects", withRole("observer", handleListProjects(of)))

	// Pipeline (auth + role)
	mux.HandleFunc("GET /api/pipelines/{id}", withRole("observer", handleGetPipeline(of)))
	mux.HandleFunc("POST /api/projects/{id}/pipelines", withRole("pm", handleCreatePipeline(of)))

	// Gate approval (dev_lead)
	mux.HandleFunc("GET /api/review-inbox", withRole("dev_lead", handleReviewInbox(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}", withRole("dev_lead", handleApproveGate(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}/reject", withRole("dev_lead", handleRejectGate(of)))

	// Pipeline fork (pm)
	mux.HandleFunc("POST /api/pipelines/{id}/fork", withRole("pm", handleForkPipeline(of)))

	// Token/Cost (pm)
	mux.HandleFunc("GET /api/projects/{id}/token-usage", withRole("pm", handleTokenUsage(of)))
	mux.HandleFunc("GET /api/projects/{id}/token-budget", withRole("pm", handleTokenBudget(of)))

	// Models (observer)
	mux.HandleFunc("GET /api/models", withRole("observer", handleListModels(of)))

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
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}
		if req.Username == "" {
			writeError(w, 400, "username required")
			return
		}
		role := req.Role
		if role == "" {
			role = "pm" // default for dev mode
		}
		token, err := jwtSvc.Issue(req.Username, role, "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, token)
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

func handleStatic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Static-File", "not-implemented")
		writeError(w, 404, "static files not available in dev mode (use Vite dev server)")
	}
}
