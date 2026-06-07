package domain

import (
	"sort"
	"sync"
	"time"
)

type SLOSnapshot struct {
	Total       int
	SuccessRate float64
	P95Ms       int
}

type SLOTracker struct {
	mu        sync.RWMutex
	total     int
	success   int
	durations []time.Duration
}

func NewSLOTracker() *SLOTracker {
	return &SLOTracker{}
}

func (s *SLOTracker) RecordPipeline(duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	if success {
		s.success++
	}
	s.durations = append(s.durations, duration)
}

func (s *SLOTracker) Snapshot() SLOSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.total == 0 {
		return SLOSnapshot{SuccessRate: 100.0}
	}

	rate := float64(s.success) / float64(s.total) * 100

	return SLOSnapshot{
		Total:       s.total,
		SuccessRate: rate,
		P95Ms:       s.p95Locked(),
	}
}

// P95 returns the 95th percentile pipeline duration in milliseconds. It is
// safe to call concurrently with RecordPipeline. Returns 0 when no samples
// have been recorded.
func (s *SLOTracker) P95() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.p95Locked()
}

// p95Locked computes the 95th percentile of the current durations buffer. The
// caller must hold s.mu (read or write).
func (s *SLOTracker) p95Locked() int {
	if len(s.durations) == 0 {
		return 0
	}
	// Copy and sort so we do not mutate the internal buffer (also safe under
	// RLock).
	sorted := make([]time.Duration, len(s.durations))
	copy(sorted, s.durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	// Index of the 95th percentile (ceiling), clamped into range.
	idx := int(0.95 * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return int(sorted[idx].Milliseconds())
}
