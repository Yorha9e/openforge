package featureflags

import (
	"fmt"
	"time"
)

// Canonical status values for the feature flag state machine.
const (
	StatusExperimental = "experimental"
	StatusBeta         = "beta"
	StatusStable       = "stable"
	StatusDeprecated   = "deprecated"
	StatusExpired      = "expired"
)

// allowedTransitions defines the state machine for feature flag lifecycle.
// Forward progression: experimental → beta → stable → deprecated → expired.
// The only lateral moves are experimental→deprecated and beta→experimental
// (rollback), and beta→deprecated. From "expired" there is no exit — the
// flag is terminal.
var allowedTransitions = map[string]map[string]bool{
	StatusExperimental: {
		StatusBeta:       true,
		StatusDeprecated: true,
	},
	StatusBeta: {
		StatusExperimental: true, // rollback
		StatusStable:       true,
		StatusDeprecated:   true,
	},
	StatusStable: {
		StatusDeprecated: true,
	},
	StatusDeprecated: {
		StatusExpired: true,
	},
	StatusExpired: {}, // terminal
}

// ValidateTransition checks that a flag can move from `from` → `to`.
// Returns nil when the transition is permitted, an error otherwise.
func ValidateTransition(from, to string) error {
	if from == to {
		return nil // no-op transition is always allowed
	}
	fromSet, ok := allowedTransitions[from]
	if !ok {
		return fmt.Errorf("unknown source status %q", from)
	}
	if !fromSet[to] {
		return fmt.Errorf("transition %q → %q is not allowed", from, to)
	}
	// Reject unknown target statuses (defense in depth).
	if _, ok := allowedTransitions[to]; !ok {
		return fmt.Errorf("unknown target status %q", to)
	}
	return nil
}

// CheckExpired returns the names of flags whose expires_at is in the past
// relative to `now` AND whose status has not already been set to "expired".
// Returns nil when there are no expired flags or when the FeatureFlags has
// no Lifecycle map.
func CheckExpired(ff *FeatureFlags, now time.Time) []string {
	if ff == nil || ff.Lifecycle == nil {
		return nil
	}
	var expired []string
	for name, lc := range ff.Lifecycle {
		if lc.ExpiresAt == nil {
			continue
		}
		if lc.Status == StatusExpired {
			continue
		}
		if lc.ExpiresAt.Before(now) {
			expired = append(expired, name)
		}
	}
	return expired
}
