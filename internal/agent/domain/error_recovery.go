package domain

import "fmt"

type FailureCode string

const (
	FailModelHallucination FailureCode = "MODEL_HALLUCINATION"
	FailPromptWeakness     FailureCode = "PROMPT_WEAKNESS"
	FailDependencyConflict FailureCode = "DEPENDENCY_CONFLICT"
	FailSandboxTimeout     FailureCode = "SANDBOX_TIMEOUT"
	FailRepoBug            FailureCode = "REPO_BUG"
	FailContextOverflow    FailureCode = "CONTEXT_OVERFLOW"
	FailTokenQuotaExceeded FailureCode = "TOKEN_QUOTA_EXCEEDED"
	FailAPITimeout         FailureCode = "API_TIMEOUT"
	FailRateLimited        FailureCode = "RATE_LIMITED"
	FailOverloaded         FailureCode = "OVERLOADED"
	FailUnknown            FailureCode = "UNKNOWN"
)

type RecoveryAction int

const (
	ActionRetry          RecoveryAction = iota
	ActionCompress
	ActionDowngradeModel
	ActionSelfRepair
	ActionClarify
	ActionLockVersion
	ActionEscalate
)

func (a RecoveryAction) String() string {
	switch a {
	case ActionRetry:
		return "RETRY"
	case ActionCompress:
		return "COMPRESS"
	case ActionDowngradeModel:
		return "DOWNGRADE_MODEL"
	case ActionSelfRepair:
		return "SELF_REPAIR"
	case ActionClarify:
		return "CLARIFY"
	case ActionLockVersion:
		return "LOCK_VERSION"
	case ActionEscalate:
		return "ESCALATE"
	default:
		return "UNKNOWN"
	}
}

type RecoveryResult struct {
	Action  RecoveryAction
	Message string
}

// ClassifyAndRecover maps a failure to the appropriate recovery layer.
// Returns ActionEscalate if no automatic recovery applies.
func ClassifyAndRecover(code FailureCode, attempt int) RecoveryResult {
	if attempt < 0 {
		attempt = 0
	}

	// Layer 1: TRANSIENT — retry with backoff
	switch code {
	case FailAPITimeout, FailRateLimited, FailOverloaded:
		if attempt >= 3 {
			return RecoveryResult{ActionEscalate, fmt.Sprintf("TRANSIENT exhausted after %d retries", attempt)}
		}
		return RecoveryResult{ActionRetry, fmt.Sprintf("attempt %d/3", attempt+1)}
	}

	// Layer 2: DEGRADABLE
	if code == FailContextOverflow && attempt == 0 {
		return RecoveryResult{ActionCompress, "context compressed, retrying"}
	}
	if code == FailTokenQuotaExceeded && attempt == 0 {
		return RecoveryResult{ActionDowngradeModel, "downgrading to cheaper model"}
	}

	// Layer 3: RECOVERABLE
	switch code {
	case FailModelHallucination:
		return RecoveryResult{ActionSelfRepair, "re-generating from repo baseline"}
	case FailPromptWeakness:
		return RecoveryResult{ActionClarify, "asking PM for clarification"}
	case FailDependencyConflict:
		return RecoveryResult{ActionLockVersion, "locking dependency versions"}
	}

	// Layer 4: FATAL
	return RecoveryResult{ActionEscalate, fmt.Sprintf("FATAL: %s", code)}
}

// IsRetryable returns true if the failure can be retried at a lower layer.
func IsRetryable(code FailureCode) bool {
	switch code {
	case FailAPITimeout, FailRateLimited, FailOverloaded,
		FailContextOverflow, FailTokenQuotaExceeded,
		FailModelHallucination, FailPromptWeakness, FailDependencyConflict:
		return true
	default:
		return false
	}
}
