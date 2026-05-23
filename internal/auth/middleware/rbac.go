package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"openforge/internal/auth/domain"
)

// RequireRole checks whether the user in context has the required role.
// Admin users bypass all role checks.
func RequireRole(ctx context.Context, required string) bool {
	role, ok := ctx.Value(domain.UserRoleContextKey).(string)
	if !ok || role == "" {
		return false
	}
	if role == "admin" {
		return true
	}
	return role == required
}

// RequireRoleMiddleware returns a middleware that enforces role-based access.
func RequireRoleMiddleware(requiredRole string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !RequireRole(r.Context(), requiredRole) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
			return
		}
		next(w, r)
	}
}
