package kernel

import "testing"

func TestLevelIsValid(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  bool
	}{
		{"L1 valid", LevelL1, true},
		{"L2 valid", LevelL2, true},
		{"L3 valid", LevelL3, true},
		{"L4 valid", LevelL4, true},
		{"empty invalid", Level(""), false},
		{"unknown invalid", Level("L5"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.IsValid(); got != tt.want {
				t.Errorf("Level(%q).IsValid() = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestPipelineStatusIsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status PipelineStatus
		want   bool
	}{
		{"completed is terminal", StatusCompleted, true},
		{"rejected is terminal", StatusRejected, true},
		{"token_exceeded is terminal", StatusTokenExceeded, true},
		{"cancelled is terminal", StatusCancelled, true},
		{"running is not terminal", StatusRunning, false},
		{"pending is not terminal", StatusPending, false},
		{"paused is not terminal", StatusPaused, false},
		{"awaiting_review is not terminal", StatusAwaitingReview, false},
		{"dormant is not terminal", StatusDormant, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Errorf("%s.IsTerminal() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
