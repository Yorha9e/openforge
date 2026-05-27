package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"openforge/internal/shared/featureflags"
	"openforge/internal/shared/profile"
)

// handleGetFeatureFlags returns the current feature flag state.
func handleGetFeatureFlags(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, of.FeatureFlags)
	}
}

// handleUpdateFeatureFlags accepts a full FeatureFlags JSON body and persists
// all 4 flags to the DB in a single transaction, then syncs the in-memory state.
// REVISED (G1+G7): Added sync.RWMutex for concurrency safety + batch UPSERT in one tx.
func handleUpdateFeatureFlags(of *profile.OpenForge, store *featureflags.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req featureflags.FeatureFlags
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}

		// Persist all 4 flags in a single DB transaction.
		if err := store.SaveAll(r.Context(), &req); err != nil {
			slog.Error("featureflags save failed", "error", err)
			writeError(w, 500, sanitizeError(err))
			return
		}

		// Sync in-memory state with write lock (G1: concurrency safe).
		of.FeatureFlags.Lock()
		of.FeatureFlags.EnterprisePlatform = req.EnterprisePlatform
		of.FeatureFlags.ComplianceSuite = req.ComplianceSuite
		of.FeatureFlags.ProductionOps = req.ProductionOps
		of.FeatureFlags.DistributionArtifacts = req.DistributionArtifacts
		of.FeatureFlags.Unlock()

		writeJSON(w, 200, of.FeatureFlags)
	}
}
