package server

import (
	"net/http"

	"openforge/internal/compliance"
	"openforge/internal/shared/profile"
)

// handleGetComplianceReports returns the 4 monthly compliance reports
// (audit, access, data, license) for the requested month. Admin-only —
// the route registration enforces the role check.
func handleGetComplianceReports(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		month := r.PathValue("month")
		if month == "" {
			writeError(w, http.StatusBadRequest, "month is required (YYYY-MM)")
			return
		}

		// The report generator is wired in the compliance package; if no
		// database is configured we still return a structured empty
		// response so the admin UI can render gracefully.
		gen := compliance.NewReportGenerator(of.DB, nil)
		ctx := r.Context()

		reports := make([]*compliance.Report, 0, len(compliance.AllReportTypes))
		for _, t := range compliance.AllReportTypes {
			rep, err := gen.Generate(ctx, t, month)
			if err != nil {
				// Surface the error on the report it came from; keep the
				// other reports in the payload so admins can still
				// review what succeeded.
				reports = append(reports, &compliance.Report{
					Type:  t,
					Month: month,
					Title: string(t) + " (error: " + err.Error() + ")",
				})
				continue
			}
			reports = append(reports, rep)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"month":   month,
			"reports": reports,
		})
	}
}
