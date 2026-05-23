package llm

import "testing"

func TestRegistry_Lookup(t *testing.T) {
	r := NewRegistry()
	e, err := r.Lookup("sonnet")
	if err != nil {
		t.Fatalf("Lookup(sonnet): %v", err)
	}
	if e.Provider != "anthropic" {
		t.Errorf("provider = %s, want anthropic", e.Provider)
	}
	if len(e.Fallback) != 2 {
		t.Errorf("fallback count = %d, want 2", len(e.Fallback))
	}
}

func TestRegistry_Lookup_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Lookup("nonexistent")
	if err == nil {
		t.Error("expected error for unknown model")
	}
}
