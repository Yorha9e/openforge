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

// featureFlagsUpdateRequest mirrors FeatureFlags but accepts the optional
// `lifecycle` map of per-flag metadata. Defined separately so the request
// body format stays backward compatible (existing PUTs that only send the
// 4 booleans still work).
type featureFlagsUpdateRequest struct {
	EnterprisePlatform    bool                                    `json:"enterprise_platform"`
	ComplianceSuite       bool                                    `json:"compliance_suite"`
	ProductionOps         bool                                    `json:"production_ops"`
	DistributionArtifacts bool                                    `json:"distribution_artifacts"`
	Lifecycle             map[string]featureflags.FlagLifecycle   `json:"lifecycle,omitempty"`
}

// handleUpdateFeatureFlags accepts a full FeatureFlags JSON body and persists
// all 4 flags + lifecycle to the DB in a single transaction, then syncs the
// in-memory state. Per-flag status changes are validated against the
// lifecycle state machine (ValidateTransition) and rejected with 400 when
// the move is not allowed.
func handleUpdateFeatureFlags(of *profile.OpenForge, store *featureflags.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req featureFlagsUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}

		// Validate lifecycle state-machine transitions against the current
		// in-memory state. Skip the check when the lifecycle entry is new
		// (no prior status).
		of.FeatureFlags.RLock()
		for name, lc := range req.Lifecycle {
			prev, hasPrev := of.FeatureFlags.Lifecycle[name]
			if !hasPrev {
				continue
			}
			if err := featureflags.ValidateTransition(prev.Status, lc.Status); err != nil {
				of.FeatureFlags.RUnlock()
				writeError(w, 400, err.Error())
				return
			}
		}
		of.FeatureFlags.RUnlock()

		merged := &featureflags.FeatureFlags{
			EnterprisePlatform:    req.EnterprisePlatform,
			ComplianceSuite:       req.ComplianceSuite,
			ProductionOps:         req.ProductionOps,
			DistributionArtifacts: req.DistributionArtifacts,
			Lifecycle:             req.Lifecycle,
		}

		// Persist all 4 flags + lifecycle in a single DB transaction.
		if err := store.SaveAll(r.Context(), merged); err != nil {
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
		of.FeatureFlags.Lifecycle = req.Lifecycle
		of.FeatureFlags.Unlock()

		writeJSON(w, 200, of.FeatureFlags)
	}
}
