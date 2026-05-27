package domain

import (
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
	p95 := 0
	if len(s.durations) > 0 {
		// Just a simple estimate
		p95 = int(s.durations[len(s.durations)*95/100].Milliseconds())
	}

	return SLOSnapshot{
		Total:       s.total,
		SuccessRate: rate,
		P95Ms:       p95,
	}
}
