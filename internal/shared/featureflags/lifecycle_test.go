package featureflags

import (
	"testing"
	"time"
)

// TestValidateTransition_ExperimentalToBeta is the canonical happy path.
func TestValidateTransition_ExperimentalToBeta(t *testing.T) {
	if err := ValidateTransition(StatusExperimental, StatusBeta); err != nil {
		t.Fatalf("experimental→beta should be allowed, got: %v", err)
	}
}

// TestValidateTransition_StableToExperimental_Rejected guards against
// skipping backwards in the lifecycle (stable is later in the progression
// than experimental, so the move must be rejected).
func TestValidateTransition_StableToExperimental_Rejected(t *testing.T) {
	if err := ValidateTransition(StatusStable, StatusExperimental); err == nil {
		t.Fatal("stable→experimental should be REJECTED, got nil error")
	}
}

// TestValidateTransition_UnknownStatus_Rejected ensures typos in status
// don't silently pass the state machine.
func TestValidateTransition_UnknownStatus_Rejected(t *testing.T) {
	if err := ValidateTransition("alpha", StatusBeta); err == nil {
		t.Fatal("alpha→beta should be REJECTED, got nil error")
	}
	if err := ValidateTransition(StatusBeta, "alpha"); err == nil {
		t.Fatal("beta→alpha should be REJECTED, got nil error")
	}
}

// TestCheckExpired_FlagsExpired verifies that flags whose expires_at is in
// the past are reported by name, and that flags without expires_at or with
// expires_at in the future are NOT reported.
func TestCheckExpired_FlagsExpired(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	ff := &FeatureFlags{
		Lifecycle: map[string]FlagLifecycle{
			"expired_flag":         {Status: "experimental", ExpiresAt: &past},
			"alive_flag":           {Status: "beta", ExpiresAt: &future},
			"no_expiry_flag":       {Status: "stable"}, // ExpiresAt is nil
			"already_expired_state": {Status: "expired", ExpiresAt: &past}, // status already says expired
		},
	}

	got := CheckExpired(ff, now)

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 expired flag, got %d: %v", len(got), got)
	}
	if got[0] != "expired_flag" {
		t.Errorf("expected 'expired_flag', got %q", got[0])
	}
}
