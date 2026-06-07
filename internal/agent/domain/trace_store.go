package domain

import (
	"context"
	"sync"
	"time"
)

// TraceEvent is a single debugging trace entry emitted during pipeline execution.
// It is intentionally lightweight: a stage label, an event name, an opaque
// JSON-serialisable payload, and a server-side timestamp.  Payloads should
// be small and deterministic — the trace store is a 30-day hot cache, not
// a full audit log.
type TraceEvent struct {
	PipelineID string
	Stage      string
	Event      string
	Payload    map[string]any
	Timestamp  time.Time
}

// TraceStore persists per-pipeline trace events.  Implementations MUST be
// safe for concurrent use by multiple goroutines and SHOULD support a
// bounded retention window (the 30-day default is enforced at the handler
// layer; concrete stores may keep more or less).
type TraceStore interface {
	// Append records a single event.  Implementations may batch internally.
	Append(ctx context.Context, ev TraceEvent) error
	// ListSince returns every event with Timestamp >= since, ordered
	// ascending by time.  An empty since (zero time) returns all events.
	ListSince(ctx context.Context, pipelineID string, since time.Time) ([]TraceEvent, error)
}

// --- In-memory implementation (used by tests and single-process runs) ---

// MemTraceStore is a concurrency-safe in-memory TraceStore.  It is the
// default fallback when no PG adapter is wired (Path-D T6).
type MemTraceStore struct {
	mu     sync.RWMutex
	events map[string][]TraceEvent
}

// NewMemTraceStore returns an empty in-memory trace store.
func NewMemTraceStore() *MemTraceStore {
	return &MemTraceStore{events: make(map[string][]TraceEvent)}
}

// Append records the event under its PipelineID.
func (s *MemTraceStore) Append(_ context.Context, ev TraceEvent) error {
	if ev.PipelineID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[ev.PipelineID] = append(s.events[ev.PipelineID], ev)
	return nil
}

// ListSince returns all events for a pipeline with Timestamp >= since,
// sorted ascending by time.  When since is the zero value, every stored
// event is returned.
func (s *MemTraceStore) ListSince(_ context.Context, pipelineID string, since time.Time) ([]TraceEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.events[pipelineID]
	out := make([]TraceEvent, 0, len(all))
	for _, ev := range all {
		if !since.IsZero() && ev.Timestamp.Before(since) {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}
