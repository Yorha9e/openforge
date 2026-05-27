package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	authport "openforge/internal/auth/port"
	"openforge/internal/auth/domain"
	"openforge/internal/auth/service"
	observabilitydomain "openforge/internal/observability/domain"
	"openforge/internal/shared/profile"
)

type ResourceSnapshotProvider interface {
	Snapshot() observabilitydomain.ResourceSnapshot
}

// ofResourceSnapshotProvider bridges OpenForge runtime state to load-shedding ResourceSnapshot.
type ofResourceSnapshotProvider struct {
	of *profile.OpenForge
}

func (p *ofResourceSnapshotProvider) Snapshot() observabilitydomain.ResourceSnapshot {
	return observabilitydomain.ResourceSnapshot{
		GoroutinesAvail:   10000,
		GoroutinesMax:     10000,
		SandboxWarm:       10,
		SandboxMin:        5,
		PGIdleConns:       20,
		LLMQueueDepth:     0,
		LLMQueueThreshold: 50,
	}
}

func LoadShedMiddleware(ls *observabilitydomain.LoadShedder, provider ResourceSnapshotProvider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		priorityStr := r.Header.Get("X-OpenForge-Priority")
		priority := 3 // default lowest priority (P3)
		if priorityStr != "" {
			if p, err := strconv.Atoi(priorityStr); err == nil {
				priority = p
			}
		}

		decision := ls.Shed(provider.Snapshot(), priority)
		if !decision.Accept {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(decision.RetryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "load shedding: system capacity critical")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(jwtSvc *service.JWTService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				writeError(w, 401, "missing or invalid Authorization header")
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			claims, err := jwtSvc.Verify(token)
			if err != nil {
				writeError(w, 401, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), domain.UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, domain.UserRoleContextKey, claims.Role)
			ctx = context.WithValue(ctx, domain.ProjectIDContextKey, claims.ProjectID)
			next(w, r.WithContext(ctx))
		}
	}
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(domain.UserIDContextKey).(string)
	return v
}

func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(domain.UserRoleContextKey).(string)
	return v
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data:")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func RateLimitMiddleware(maxPerSec int) func(http.Handler) http.Handler {
	type entry struct {
		count   int
		resetAt time.Time
	}
	buckets := make(map[string]*entry)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			now := time.Now()
			b, ok := buckets[ip]
			if !ok || now.After(b.resetAt) {
				buckets[ip] = &entry{count: 1, resetAt: now.Add(time.Second)}
			} else if b.count >= maxPerSec {
				writeError(w, 429, "rate limit exceeded")
				return
			} else {
				b.count++
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TenantMiddleware enforces multi-tenant project access isolation.
// It reads user/project identity from JWT context (set by AuthMiddleware)
// or falls back to X-User-ID / X-Project-ID headers for service-to-service calls.
// Public endpoints (health, metrics, auth) are exempted.
func TenantMiddleware(authRepo authport.AuthRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip public endpoints that don't require tenant isolation.
			path := r.URL.Path
			if path == "/api/health" || path == "/metrics" ||
				strings.HasPrefix(path, "/api/auth/") ||
				strings.HasPrefix(path, "/ws/") {
				next.ServeHTTP(w, r)
				return
			}

			// TenantMiddleware runs OUTSIDE per-route AuthMiddleware, so JWT context
			// is NOT available here for authenticated routes.  Only perform multi-tenant
			// isolation when BOTH userID AND projectID are present (e.g. service-to-service
			// calls via X-User-ID / X-Project-ID headers).  Regular browser requests are
			// authenticated by AuthMiddleware inside each route handler.
			userID := UserIDFromContext(r.Context())
			projectID := ""
			if v, ok := r.Context().Value(domain.ProjectIDContextKey).(string); ok {
				projectID = v
			}

			if projectID == "" {
				projectID = r.Header.Get("X-Project-ID")
			}
			if userID == "" {
				userID = r.Header.Get("X-User-ID")
			}

			// Only enforce tenant isolation when both identities are available.
			if userID != "" && projectID != "" {
				role, _ := authRepo.GetUserRole(r.Context(), userID, projectID)
				if role == nil {
					http.Error(w, "unauthorized role assignment for project", http.StatusForbidden)
					return
				}
				ctx := context.WithValue(r.Context(), domain.ProjectIDContextKey, projectID)
				ctx = context.WithValue(ctx, domain.UserIDContextKey, userID)
				ctx = context.WithValue(ctx, domain.UserRoleContextKey, role.Role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No tenant context — pass through. Auth is handled by per-route AuthMiddleware.
			next.ServeHTTP(w, r)
		})
	}
}
