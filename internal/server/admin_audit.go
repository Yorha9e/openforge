package server

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"openforge/internal/shared/profile"
)

// AuditLogEntry represents a single audit log record.
type AuditLogEntry struct {
	Event     string    `json:"event"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	ProjectID string    `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
}

// handleAuditExport returns an HTTP handler that exports audit logs as CSV.
// Only accessible by admin users when compliance_suite feature flag is enabled.
func handleAuditExport(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		of.FeatureFlags.RLock()
		enabled := of.FeatureFlags.ComplianceSuite
		of.FeatureFlags.RUnlock()
		if !enabled {
			writeError(w, 404, "feature disabled")
			return
		}

		if of.DB == nil {
			http.Error(w, "database not available", http.StatusInternalServerError)
			return
		}

		// Query audit logs (last 10,000 entries)
		query := `
			SELECT event, actor, action, resource, result, project_id, created_at
			FROM audit_log
			ORDER BY created_at DESC
			LIMIT 10000
		`

		rows, err := of.DB.QueryContext(r.Context(), query)
		if err != nil {
			slog.Error("failed to query audit logs", "error", err)
			http.Error(w, "failed to query audit logs", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Set CSV headers
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=audit_log_%s.csv", time.Now().Format("20060102_150405")))

		// Create CSV writer
		writer := csv.NewWriter(w)
		defer writer.Flush()

		// Write CSV header
		header := []string{"event", "actor", "action", "resource", "result", "project_id", "created_at"}
		if err := writer.Write(header); err != nil {
			slog.Error("failed to write CSV header", "error", err)
			return
		}

		// Write rows
		count := 0
		for rows.Next() {
			var entry AuditLogEntry
			if err := rows.Scan(
				&entry.Event,
				&entry.Actor,
				&entry.Action,
				&entry.Resource,
				&entry.Result,
				&entry.ProjectID,
				&entry.CreatedAt,
			); err != nil {
				slog.Error("failed to scan audit log row", "error", err)
				continue
			}

			record := []string{
				entry.Event,
				entry.Actor,
				entry.Action,
				entry.Resource,
				entry.Result,
				entry.ProjectID,
				entry.CreatedAt.Format(time.RFC3339),
			}

			if err := writer.Write(record); err != nil {
				slog.Error("failed to write CSV record", "error", err)
				return
			}
			count++
		}

		if err := rows.Err(); err != nil {
			slog.Error("error iterating audit log rows", "error", err)
			return
		}

		slog.Info("audit log exported", "records", count)
	}
}

// Verify that handleAuditExport returns http.HandlerFunc.
var _ http.HandlerFunc = handleAuditExport(nil)