package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	agentdomain "openforge/internal/agent/domain"
	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"
)

// Default trace retention window: the handler clamps `since` to
// `now - 30d` so a missing/bogus client value can never stream the
// whole history.
const defaultTraceWindow = 30 * 24 * time.Hour

// handleGetDebugTrace serves GET /api/debug/trace/{id}.
//
// Authorization: admin only (the debug endpoint is intentionally
// tighter than the replay endpoint; replay allows any authenticated
// observer, but only admins can pull raw per-event trace payloads).
// Time window: every caller is implicitly limited to the last 30 days;
// clients may pass ?since=<RFC3339> to narrow further.
func handleGetDebugTrace(of *profile.OpenForge, jwtSvc *service.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Authenticate (re-uses the standard AuthMiddleware contract).
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if _, err := jwtSvc.Verify(token); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// 2. Authorize (admin only).
		if UserRoleFromContext(r.Context()) != "admin" {
			writeError(w, http.StatusForbidden, "forbidden: admin role required")
			return
		}

		pipelineID := r.PathValue("id")
		if pipelineID == "" {
			writeError(w, http.StatusBadRequest, "missing pipeline id")
			return
		}

		// 3. Resolve time window. Default = now - 30d.
		since := time.Now().UTC().Add(-defaultTraceWindow)
		if raw := r.URL.Query().Get("since"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid since parameter (expected RFC3339)")
				return
			}
			// Client can narrow but not widen the 30-day window.
			if parsed.After(since) {
				since = parsed
			}
		}

		// 4. Stream the trace.
		if of.TraceStore == nil {
			writeError(w, http.StatusServiceUnavailable, "trace store not configured")
			return
		}
		events, err := of.TraceStore.ListSince(r.Context(), pipelineID, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if events == nil {
			events = []agentdomain.TraceEvent{}
		}
		writeJSON(w, http.StatusOK, events)
	}
}

// errNotPipelineOwner is reserved for future richer error responses; kept
// here so we don't sprinkle magic strings in callers.
var errNotPipelineOwner = errors.New("not pipeline owner")
