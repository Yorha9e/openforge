package domain

import (
	"testing"
	"time"
)

func TestLoadShed_Normal(t *testing.T) {
	ls := NewLoadShedder()
	snap := ResourceSnapshot{
		GoroutinesAvail:   8000,
		GoroutinesMax:     8000,
		SandboxWarm:       10,
		SandboxMin:        5,
		PGIdleConns:       20,
		LLMQueueDepth:     0,
		LLMQueueThreshold: 50,
	}
	d := ls.Shed(snap, 3)
	if !d.Accept {
		t.Error("expected accept for P3 at NORMAL")
	}
	if d.Level != CapacityNormal {
		t.Errorf("expected NORMAL, got %s", d.Level)
	}
	if d.RetryAfter != 0 {
		t.Errorf("expected 0 retry, got %v", d.RetryAfter)
	}
}

func TestLoadShed_LLMQueueFull(t *testing.T) {
	ls := NewLoadShedder()
	snap := ResourceSnapshot{
		GoroutinesAvail:   8000,
		GoroutinesMax:     8000,
		SandboxWarm:       10,
		SandboxMin:        5,
		PGIdleConns:       20,
		LLMQueueDepth:     60,
		LLMQueueThreshold: 50,
	}
	d := ls.Shed(snap, 1) // P1 rejected at CRITICAL
	if d.Accept {
		t.Error("expected reject when LLM queue is full")
	}
	if d.Level != CapacityCritical {
		t.Errorf("expected CRITICAL, got %s", d.Level)
	}
}

func TestLoadShed_RejectionByPriority(t *testing.T) {
	ls := NewLoadShedder()
	// 25% capacity → WARNING
	snap := ResourceSnapshot{
		GoroutinesAvail:   2500,
		GoroutinesMax:     10000,
		SandboxWarm:       3,
		SandboxMin:        10,
		PGIdleConns:       6,
		LLMQueueDepth:     0,
		LLMQueueThreshold: 50,
	}
	// P3 rejected at WARNING
	d := ls.Shed(snap, 3)
	if d.Accept {
		t.Error("WARNING should reject P3")
	}
	if d.Level != CapacityWarning {
		t.Errorf("expected WARNING, got %s", d.Level)
	}
	// P0 accepted at WARNING
	d2 := ls.Shed(snap, 0)
	if !d2.Accept {
		t.Error("WARNING should accept P0")
	}
}

func TestLoadShed_CriticalAcceptsOnlyP0(t *testing.T) {
	ls := NewLoadShedder()
	// near-0% capacity → CRITICAL
	snap := ResourceSnapshot{
		GoroutinesAvail:   500,
		GoroutinesMax:     10000,
		SandboxWarm:       1,
		SandboxMin:        10,
		PGIdleConns:       3,
		LLMQueueDepth:     60,
		LLMQueueThreshold: 50,
	}
	// P1 rejected
	d := ls.Shed(snap, 1)
	if d.Accept {
		t.Error("CRITICAL should reject P1")
	}
	if d.Level != CapacityCritical {
		t.Errorf("expected CRITICAL, got %s", d.Level)
	}
	// P0 accepted
	d2 := ls.Shed(snap, 0)
	if !d2.Accept {
		t.Error("CRITICAL should accept P0")
	}
	if d2.RetryAfter != 30*time.Second {
		t.Errorf("CRITICAL retry_after should be 30s, got %v", d2.RetryAfter)
	}
}

func TestLoadShed_ZeroGuard(t *testing.T) {
	ls := NewLoadShedder()
	// All zero guards → 0% → CRITICAL
	snap := ResourceSnapshot{}
	d := ls.Shed(snap, 1) // P1 rejected at CRITICAL
	if d.Accept {
		t.Error("expected reject for zero-guard input")
	}
	if d.Level != CapacityCritical {
		t.Errorf("expected CRITICAL, got %s", d.Level)
	}
}

func TestLoadShed_CapacityLevelString(t *testing.T) {
	if CapacityNormal.String() != "NORMAL" {
		t.Errorf("unexpected Normal string: %s", CapacityNormal.String())
	}
	if CapacityWarning.String() != "WARNING" {
		t.Errorf("unexpected Warning string: %s", CapacityWarning.String())
	}
	if CapacityCritical.String() != "CRITICAL" {
		t.Errorf("unexpected Critical string: %s", CapacityCritical.String())
	}
	if CapacityLevel(99).String() != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for invalid level")
	}
}
