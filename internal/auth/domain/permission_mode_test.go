package domain

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		pc   PermissionContext
		want PermissionDecision
	}{
		{"bypass always allows", PermissionContext{Mode: PermissionModeBypass}, DecisionAllow},
		{"plan allows read-only", PermissionContext{Mode: PermissionModePlan, IsReadOnly: true}, DecisionAllow},
		{"plan blocks write", PermissionContext{Mode: PermissionModePlan, IsReadOnly: false}, DecisionAskGate},
		{"auto allows read-only", PermissionContext{Mode: PermissionModeAuto, IsReadOnly: true}, DecisionAllow},
		{"auto allows file-in-lock", PermissionContext{Mode: PermissionModeAuto, FileInLock: true}, DecisionAllow},
		{"auto allows whitelisted", PermissionContext{Mode: PermissionModeAuto, FileInWhitelist: true}, DecisionAllow},
		{"auto blocks unknown write", PermissionContext{Mode: PermissionModeAuto}, DecisionAskGate},
		{"default always asks gate", PermissionContext{Mode: PermissionModeDefault}, DecisionAskGate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.pc)
			if got != tt.want {
				t.Errorf("Classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectMode(t *testing.T) {
	tests := []struct {
		level, stage string
		want         PermissionMode
	}{
		{"L1", "impl", PermissionModeAuto},
		{"L2", "decompose", PermissionModeAuto},
		{"L3", "impl", PermissionModeDefault},
		{"L4", "impl", PermissionModeDefault},
		{"L3", "clarify", PermissionModePlan},
		{"L4", "clarify", PermissionModePlan},
	}
	for _, tt := range tests {
		t.Run(tt.level+"/"+tt.stage, func(t *testing.T) {
			got := SelectMode(tt.level, tt.stage)
			if got != tt.want {
				t.Errorf("SelectMode(%q, %q) = %v, want %v", tt.level, tt.stage, got, tt.want)
			}
		})
	}
}
