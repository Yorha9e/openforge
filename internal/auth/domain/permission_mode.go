package domain

// PermissionMode defines how the pipeline handles gate approval.
type PermissionMode int

const (
	PermissionModeDefault PermissionMode = iota // full gate approval required
	PermissionModeBypass                        // no gate, full write access
	PermissionModeAuto                          // read-only, auto-approve
	PermissionModePlan                          // read-only, plan only
)

// SelectMode returns the permission mode for a given pipeline level and stage.
func SelectMode(level, stage string) PermissionMode {
	// L1/L2: bypass gate
	if level == "L1" || level == "L2" {
		return PermissionModeBypass
	}
	// Verify stage: always default (never auto-close)
	if stage == "verify" {
		return PermissionModeDefault
	}
	// Deploy stage: plan mode (read-only review)
	if stage == "deploy" {
		return PermissionModePlan
	}
	// L3/L4 non-verify: default (gate required)
	return PermissionModeDefault
}
