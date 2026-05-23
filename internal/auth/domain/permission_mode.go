package domain

// PermissionMode defines how the pipeline handles gate approval.
type PermissionMode int

const (
	PermissionModeDefault PermissionMode = iota // full gate approval required
	PermissionModeBypass                        // no gate, full write access
	PermissionModeAuto                          // read-only, auto-approve
	PermissionModePlan                          // read-only, plan only
)

// PermissionDecision is the outcome of a permission check.
type PermissionDecision string

const (
	DecisionAllow   PermissionDecision = "allow"
	DecisionAskGate PermissionDecision = "ask_gate"
)

// PermissionContext carries the full state for a Classify call.
type PermissionContext struct {
	Mode           PermissionMode
	IsReadOnly     bool
	FileInLock     bool
	FileInWhitelist bool
}

// Classify determines the permission decision based on mode and context.
func Classify(pc PermissionContext) PermissionDecision {
	switch pc.Mode {
	case PermissionModeBypass:
		return DecisionAllow
	case PermissionModePlan:
		if pc.IsReadOnly {
			return DecisionAllow
		}
		return DecisionAskGate
	case PermissionModeAuto:
		if pc.IsReadOnly || pc.FileInLock || pc.FileInWhitelist {
			return DecisionAllow
		}
		return DecisionAskGate
	case PermissionModeDefault:
		return DecisionAskGate
	default:
		return DecisionAskGate
	}
}

// SelectMode returns the permission mode for a given pipeline level and stage.
func SelectMode(level, stage string) PermissionMode {
	// L1/L2: auto mode (read-only access, gate bypass for known files)
	if level == "L1" || level == "L2" {
		return PermissionModeAuto
	}
	// Verify stage: always default (never auto-close)
	if stage == "verify" {
		return PermissionModeDefault
	}
	// Deploy stage: plan mode (read-only review)
	if stage == "deploy" {
		return PermissionModePlan
	}
	// Clarify stage: plan mode (read-only review, gate required for writes)
	if stage == "clarify" {
		return PermissionModePlan
	}
	// L3/L4 non-special: default (gate required)
	return PermissionModeDefault
}
