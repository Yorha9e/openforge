package domain

import (
	"context"
	"sync"
	"time"
)

// TraceEvent is a single persisted point in a pipeline's stream of events
// (chat deltas, tool calls, pipeline stage changes, etc.) The server emits
// TraceEvents as a side effect of dispatching WebSocket messages; on
// reconnect the client may request a replay of all events newer than its
// last-known sequence number via TraceStore.ListSince.
type TraceEvent struct {
	Seq        int64     // monotonically increasing per pipeline
	PipelineID string    // owning pipeline
	Event      string    // e.g. "chat.stream", "tool.start"
	Payload    []byte    // raw JSON payload (decoded into map[string]any at the WS layer)
	Timestamp  time.Time // when the event was recorded
}

// TraceStore persists per-pipeline TraceEvents and supports replay
// queries by sequence number. Production uses a PG-backed implementation
// (see internal/agent/adapter); tests use MemTraceStore.
type TraceStore interface {
	// Record appends a TraceEvent. The Seq field is assigned by the
	// implementation if zero.
	Record(ctx context.Context, ev TraceEvent) (TraceEvent, error)
	// ListSince returns all events for the given pipeline with
	// Seq > lastSeq, in ascending Seq order. An empty slice with
	// no error means "you're caught up".
	ListSince(ctx context.Context, pipelineID string, lastSeq int64) ([]TraceEvent, error)
}

// MemTraceStore is an in-memory TraceStore for tests. It is safe for
// concurrent use.
type MemTraceStore struct {
	mu      sync.Mutex
	Data    map[string][]TraceEvent
	nextSeq map[string]int64
}

// NewMemTraceStore returns an empty in-memory TraceStore.
func NewMemTraceStore() *MemTraceStore {
	return &MemTraceStore{
		Data:    make(map[string][]TraceEvent),
		nextSeq: make(map[string]int64),
	}
}

// Record appends a TraceEvent, assigning Seq if zero.
func (s *MemTraceStore) Record(_ context.Context, ev TraceEvent) (TraceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.Seq == 0 {
		s.nextSeq[ev.PipelineID]++
		ev.Seq = s.nextSeq[ev.PipelineID]
	} else if ev.Seq > s.nextSeq[ev.PipelineID] {
		s.nextSeq[ev.PipelineID] = ev.Seq
	}
	s.Data[ev.PipelineID] = append(s.Data[ev.PipelineID], ev)
	return ev, nil
}

// ListSince returns all events for the given pipeline with Seq > lastSeq.
func (s *MemTraceStore) ListSince(_ context.Context, pipelineID string, lastSeq int64) ([]TraceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.Data[pipelineID]
	out := make([]TraceEvent, 0, len(all))
	for _, ev := range all {
		if ev.Seq > lastSeq {
			out = append(out, ev)
		}
	}
	return out, nil
}
