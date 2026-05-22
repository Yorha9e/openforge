package server

import (
	"encoding/json"
	"net/http"

	"openforge/internal/auth/service"
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

	// Projects (authenticated)
	mux.HandleFunc("GET /api/projects", authMw(handleListProjects(of)))
	mux.HandleFunc("GET /api/projects/{id}", authMw(handleGetProject(of)))

	// Pipelines (authenticated)
	mux.HandleFunc("POST /api/projects/{id}/pipelines", authMw(handleCreatePipeline(of)))
	mux.HandleFunc("GET /api/pipelines/{id}", authMw(handleGetPipeline(of)))
	mux.HandleFunc("GET /api/pipelines/{id}/messages", authMw(handleGetMessages(of)))

	// WebSocket
	mux.HandleFunc("GET /ws/chat", authMw(handleChatWS(of, jwtSvc)))

	// Static files
	mux.HandleFunc("GET /", handleStatic())

	return CorsMiddleware(SecurityHeadersMiddleware(LoggingMiddleware(mux)))
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
		if req.Username == "" {
			writeError(w, 400, "username required")
			return
		}
		token, err := jwtSvc.Issue(req.Username, "pm", "")
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
		writeJSON(w, 200, []map[string]string{})
	}
}

func handleGetProject(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		writeJSON(w, 200, map[string]string{"id": id, "name": "Demo Project"})
	}
}

func handleCreatePipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, map[string]string{"id": "pipe-001", "status": "pending"})
	}
}

func handleGetPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		writeJSON(w, 200, map[string]string{"id": id, "status": "completed"})
	}
}

func handleGetMessages(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		writeJSON(w, 200, map[string]any{"pipeline_id": id, "messages": []any{}})
	}
}

func handleStatic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Static-File", "not-implemented")
		writeError(w, 404, "static files not available in dev mode (use Vite dev server)")
	}
}
