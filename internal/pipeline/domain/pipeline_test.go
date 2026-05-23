package domain

import "testing"

func TestPipelineStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		action    string
		want      string
		wantErr   bool
	}{
		// Normal forward flow
		{"pending -> running", "pending", "start", "running", false},
		{"running -> awaiting_review", "running", "complete_stage", "awaiting_review", false},
		{"awaiting_review -> running (approved)", "awaiting_review", "gate_approve", "running", false},
		{"awaiting_review -> rejected", "awaiting_review", "gate_reject", "rejected", false},
		// Pause/resume
		{"running -> paused", "running", "pause", "paused", false},
		{"paused -> running", "paused", "resume", "running", false},
		// Cancellation
		{"running -> cancelled", "running", "cancel", "cancelled", false},
		{"pending -> cancelled", "pending", "cancel", "cancelled", false},
		// Terminal states reject transitions
		{"completed rejects start", "completed", "start", "", true},
		{"rejected rejects start", "rejected", "start", "", true},
		{"cancelled rejects start", "cancelled", "start", "", true},
		// Token exceeded check
		{"running -> token_exceeded", "running", "exceed_token", "token_exceeded", false},
		// Invalid actions
		{"pending rejects pause", "pending", "pause", "", true},
		{"awaiting_review rejects pause", "awaiting_review", "pause", "", true},
		// Backtrack
		{"running -> dormant", "running", "backtrack", "dormant", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{Status: tt.initial}
			err := p.Transition(tt.action)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Transition(%q) error = %v, wantErr = %v", tt.action, err, tt.wantErr)
			}
			if !tt.wantErr && p.Status != tt.want {
				t.Errorf("status = %q, want %q", p.Status, tt.want)
			}
		})
	}
}

func TestPipelineAdvanceStage(t *testing.T) {
	p := NewPipeline("p1", "proj1", "Test", "user1", 6, 4) // L3
	if len(p.Stages) != 6 {
		t.Fatalf("L3 should have 6 stages, got %d", len(p.Stages))
	}
	if p.CurrentStage != "clarify" {
		t.Errorf("initial stage = %q, want clarify", p.CurrentStage)
	}

	// Advance through all stages
	expected := []string{"clarify", "decompose", "impl", "test", "deploy", "verify"}
	for _, exp := range expected {
		if p.CurrentStage != exp {
			t.Errorf("stage = %q, want %q", p.CurrentStage, exp)
		}
		p.AdvanceStage()
	}
	if p.Status != "completed" {
		t.Errorf("final status = %q, want completed", p.Status)
	}
}

func TestPipelineBacktrackLimit(t *testing.T) {
	p := NewPipeline("p1", "proj1", "Test", "user1", 10, 5) // L4
	p.Status = "running"
	p.CurrentStage = "impl"

	// 3 backtracks allowed
	for i := 0; i < 3; i++ {
		err := p.Transition("backtrack")
		if err != nil {
			t.Fatalf("backtrack %d should succeed: %v", i, err)
		}
		if p.Status != "dormant" {
			t.Errorf("backtrack %d: status = %q, want dormant", i, p.Status)
		}
		p.Status = "running" // reset for next test
	}

	// 4th should fail
	err := p.Transition("backtrack")
	if err == nil {
		t.Fatal("4th backtrack should fail")
	}
	if p.BacktrackCount != 3 {
		t.Errorf("BacktrackCount = %d, want 3 (4th should not increment)", p.BacktrackCount)
	}
}

func TestPipelineBacktrackInvalidState(t *testing.T) {
	p := NewPipeline("p1", "proj1", "Test", "user1", 10, 5) // L4
	p.Status = "pending" // pending does not support backtrack

	err := p.Transition("backtrack")
	if err == nil {
		t.Fatal("backtrack from pending should fail")
	}
	if p.BacktrackCount != 0 {
		t.Errorf("BacktrackCount = %d, want 0 (invalid backtrack should not consume token)", p.BacktrackCount)
	}
}
