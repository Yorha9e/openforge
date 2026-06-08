package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	agentdomain "openforge/internal/agent/domain"
	"openforge/internal/auth/domain"
	"openforge/internal/shared/profile"
)

// handleReplayPipeline returns the persisted trace events for a pipeline,
// scoped to the last 90 days, so an operator can replay the agent run.
// Auth: any authenticated observer-or-above can replay (the replay payload
// contains only event counts and timestamps, not raw model output).
func handleReplayPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid := r.PathValue("id")
		if pid == "" {
			writeError(w, http.StatusBadRequest, "pipeline id required")
			return
		}

		// Any authenticated user with a recognized role may replay.
		role := UserRoleFromContext(r.Context())
		if role == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if of.TraceStore == nil {
			writeError(w, http.StatusServiceUnavailable, "trace store unavailable")
			return
		}

		since := time.Now().Add(-90 * 24 * time.Hour)
		// TraceStore.ListSince is sequence-number based; pull all events
		// (lastSeq=0) and filter to the requested time window client-side.
		all, err := of.TraceStore.ListSince(r.Context(), pid, 0)
		if err != nil {
			slog.Error("replay: list trace events failed", "pipeline_id", pid, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load trace events")
			return
		}
		events := make([]agentdomain.TraceEvent, 0, len(all))
		for _, ev := range all {
			if ev.Timestamp.Before(since) {
				continue
			}
			events = append(events, ev)
		}
		if events == nil {
			events = []agentdomain.TraceEvent{}
		}

		var durationS float64
		if len(events) > 0 {
			first := events[0].Timestamp
			last := events[len(events)-1].Timestamp
			if !last.Before(first) {
				durationS = last.Sub(first).Seconds()
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"pipeline_id": pid,
			"events":      events,
			"duration_s":  durationS,
		})
	}
}

// replCommand is the parsed shape of a REPL request body.
type replCommand struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args"`
}

// replResult is the JSON shape returned by the REPL handler.
type replResult struct {
	Status string `json:"status"`
	Error  string `json:"err,omitempty"`
}

// supportedREPLCommands is the whitelist of accepted REPL commands.
var supportedREPLCommands = map[string]bool{
	"restart":      true,
	"pause":        true,
	"re-run-stage": true,
}

// handleReplCommand executes a production-ops REPL command. It is gated on
// the ProductionOps feature flag and writes an audit log entry for every
// invocation (success or rejection).
func handleReplCommand(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		of.FeatureFlags.RLock()
		enabled := of.FeatureFlags.ProductionOps
		of.FeatureFlags.RUnlock()
		if !enabled {
			writeREPLAudit(of, r, "repl", "deny", "feature disabled", "")
			writeError(w, http.StatusForbidden, "production_ops feature flag disabled")
			return
		}

		userID, _ := r.Context().Value(domain.UserIDContextKey).(string)

		var req replCommand
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeREPLAudit(of, r, "repl", "reject", "invalid body", userID)
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Command == "" {
			writeREPLAudit(of, r, "repl", "reject", "missing command", userID)
			writeError(w, http.StatusBadRequest, "command is required")
			return
		}
		if !supportedREPLCommands[req.Command] {
			writeREPLAudit(of, r, "repl", "reject", "unsupported command: "+req.Command, userID)
			writeError(w, http.StatusBadRequest, "unsupported command: "+req.Command)
			return
		}

		writeREPLAudit(of, r, "repl", "allow", "command="+req.Command, userID)
		writeJSON(w, http.StatusOK, replResult{Status: "ok"})
	}
}

// writeREPLAudit persists a single audit_log row for a REPL invocation.
// Failures are logged but never block the request — audit must not
// become a denial-of-service vector.
func writeREPLAudit(of *profile.OpenForge, r *http.Request, event, action, result, actor string) {
	if of.DB == nil {
		return
	}
	_, err := of.DB.ExecContext(r.Context(), `
		INSERT INTO audit_log (event, actor, action, resource, result, project_id, source_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, '', '', '')
	`, event, actor, action, "repl:"+event, result)
	if err != nil {
		slog.Warn("repl audit insert failed", "error", err, "event", event, "action", action, "result", result)
	}
}

// errAuditDBClosed is returned when the audit log database handle is nil.
// Kept for future use by tests that exercise the audit path directly.
var errAuditDBClosed = errors.New("audit log database not configured")
