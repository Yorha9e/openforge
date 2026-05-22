package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"openforge/internal/auth/service"
)

type contextKey string

const (
	ContextUserID    contextKey = "user_id"
	ContextUserRole  contextKey = "user_role"
	ContextProjectID contextKey = "project_id"
)

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
			ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextUserRole, claims.Role)
			ctx = context.WithValue(ctx, ContextProjectID, claims.ProjectID)
			next(w, r.WithContext(ctx))
		}
	}
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextUserID).(string)
	return v
}

func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextUserRole).(string)
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
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
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
