package domain

import "testing"

func TestClassifyAndRecover(t *testing.T) {
	tests := []struct {
		name    string
		code    FailureCode
		attempt int
		want    RecoveryAction
	}{
		// Layer 1: TRANSIENT
		{"timeout retry", FailAPITimeout, 0, ActionRetry},
		{"rate limited retry", FailRateLimited, 1, ActionRetry},
		{"overloaded retry", FailOverloaded, 2, ActionRetry},
		{"timeout exhausted", FailAPITimeout, 3, ActionEscalate},
		{"rate limited exhausted", FailRateLimited, 3, ActionEscalate},
		// Layer 2: DEGRADABLE
		{"context overflow compress", FailContextOverflow, 0, ActionCompress},
		{"token quota downgrade", FailTokenQuotaExceeded, 0, ActionDowngradeModel},
		// Layer 3: RECOVERABLE
		{"hallucination self-repair", FailModelHallucination, 0, ActionSelfRepair},
		{"prompt weakness clarify", FailPromptWeakness, 0, ActionClarify},
		{"dependency conflict lock", FailDependencyConflict, 0, ActionLockVersion},
		// Layer 4: FATAL
		{"sandbox timeout fatal", FailSandboxTimeout, 0, ActionEscalate},
		{"repo bug fatal", FailRepoBug, 0, ActionEscalate},
		{"unknown fatal", FailUnknown, 0, ActionEscalate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyAndRecover(tt.code, tt.attempt)
			if got.Action != tt.want {
				t.Errorf("ClassifyAndRecover(%q, %d).Action = %v, want %v", tt.code, tt.attempt, got.Action, tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(FailAPITimeout) {
		t.Error("API timeout should be retryable")
	}
	if IsRetryable(FailSandboxTimeout) {
		t.Error("sandbox timeout should NOT be retryable after 3 attempts")
	}
	if IsRetryable(FailRepoBug) {
		t.Error("repo bugs should NOT be retryable")
	}
}
