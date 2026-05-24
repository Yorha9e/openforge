package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"openforge/internal/auth/domain"
)

// RequireRole checks whether the user in context has the required role (with hierarchy).
// Admin users bypass all role checks.
func RequireRole(ctx context.Context, required string) bool {
	return RequireOneOf(ctx, required)
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

// roleHierarchy maps a role to all roles it can act as (including itself).
var roleHierarchy = map[string][]string{
	"admin":    {"admin", "pm", "dev_lead", "dev", "observer"},
	"pm":       {"pm", "dev", "observer"},
	"dev_lead": {"dev_lead", "dev", "observer"},
	"dev":      {"dev", "observer"},
	"observer": {"observer"},
}

// RequireOneOf checks whether the user has any of the allowed roles (with hierarchy).
func RequireOneOf(ctx context.Context, allowed ...string) bool {
	role, ok := ctx.Value(domain.UserRoleContextKey).(string)
	if !ok || role == "" {
		return false
	}
	if role == "admin" {
		return true
	}
	inherited, exists := roleHierarchy[role]
	if !exists {
		return false
	}
	for _, need := range allowed {
		for _, have := range inherited {
			if have == need {
				return true
			}
		}
	}
	return false
}

// RequireRolesMiddleware returns a middleware that allows any of the specified roles.
func RequireRolesMiddleware(allowed []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !RequireOneOf(r.Context(), allowed...) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
			return
		}
		next(w, r)
	}
}
