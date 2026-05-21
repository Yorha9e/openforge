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

func TestParseContextWindow(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		wantClean  string
		wantWindow int
	}{
		{"1m suffix", "deepseek-v4-pro[1m]", "deepseek-v4-pro", 1_048_576},
		{"200k suffix", "claude-sonnet[200k]", "claude-sonnet", 204_800},
		{"128k suffix", "gpt-4[128k]", "gpt-4", 131_072},
		{"uppercase M", "model[2M]", "model", 2_097_152},
		{"uppercase K", "model[16K]", "model", 16_384},
		{"no suffix", "mimo-v2.5-pro", "mimo-v2.5-pro", 0},
		{"bracket in middle", "foo[bar]baz", "foo[bar]baz", 0},
		{"empty num", "model[k]", "model[k]", 0},
		{"zero", "model[0k]", "model[0k]", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClean, gotWindow := ParseContextWindow(tt.model)
			if gotClean != tt.wantClean {
				t.Errorf("ParseContextWindow(%q) name = %q, want %q", tt.model, gotClean, tt.wantClean)
			}
			if gotWindow != tt.wantWindow {
				t.Errorf("ParseContextWindow(%q) window = %d, want %d", tt.model, gotWindow, tt.wantWindow)
			}
		})
	}
}
