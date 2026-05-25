package domain

import (
	"context"
	"sync"
)

// TrajectoryStore persists pipeline execution trajectories (L3).
type TrajectoryStore interface {
	Record(ctx context.Context, t TrajectoryRecord) error
	ListByProject(ctx context.Context, projectID string) ([]TrajectoryRecord, error)
	GetByPipeline(ctx context.Context, pipelineID string) (*TrajectoryRecord, error)
	SimilarPatterns(ctx context.Context, projectID string, failureCodes []string, topK int) ([]TrajectoryRecord, error)
	SuccessfulTools(ctx context.Context, projectID string, stage string, topK int) ([]string, error)
	MatchedSkills(ctx context.Context, projectID string, requirement string, topK int) ([]string, error)
}

// TrajectoryRecord is a persisted pipeline execution trajectory.
type TrajectoryRecord struct {
	ID                 string
	ProjectID          string
	PipelineID         string
	StageSequence      []string
	TotalChatRounds    int
	TotalTokens        int64
	BacktrackCount     int
	RejectionCount     int
	FailureCodes       []string
	SuccessfulPatterns []string
	ToolsUsed          []string
	SkillsMatched      []string
	RequirementSummary string
}

// MemTrajectoryStore is an in-memory implementation for testing.
type MemTrajectoryStore struct {
	mu   sync.RWMutex
	data map[string][]TrajectoryRecord
}

func NewMemTrajectoryStore() *MemTrajectoryStore {
	return &MemTrajectoryStore{data: make(map[string][]TrajectoryRecord)}
}

func (s *MemTrajectoryStore) Record(_ context.Context, t TrajectoryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[t.ProjectID] = append(s.data[t.ProjectID], t)
	return nil
}

func (s *MemTrajectoryStore) ListByProject(_ context.Context, projectID string) ([]TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TrajectoryRecord, len(s.data[projectID]))
	copy(result, s.data[projectID])
	return result, nil
}

func (s *MemTrajectoryStore) GetByPipeline(_ context.Context, pipelineID string) (*TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, list := range s.data {
		for _, t := range list {
			if t.PipelineID == pipelineID {
				return &t, nil
			}
		}
	}
	return nil, nil
}

func (s *MemTrajectoryStore) SimilarPatterns(_ context.Context, projectID string, failureCodes []string, topK int) ([]TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	codeSet := make(map[string]bool, len(failureCodes))
	for _, c := range failureCodes {
		codeSet[c] = true
	}
	var matches []TrajectoryRecord
	for _, t := range s.data[projectID] {
		overlap := 0
		for _, fc := range t.FailureCodes {
			if codeSet[fc] {
				overlap++
			}
		}
		if overlap > 0 {
			matches = append(matches, t)
			if len(matches) >= topK {
				break
			}
		}
	}
	return matches, nil
}

func (s *MemTrajectoryStore) SuccessfulTools(_ context.Context, projectID string, stage string, topK int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	toolCounts := make(map[string]int)
	for _, t := range s.data[projectID] {
		if len(t.FailureCodes) == 0 {
			for _, tool := range t.ToolsUsed {
				toolCounts[tool]++
			}
		}
	}
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range toolCounts {
		sorted = append(sorted, kv{k, v})
	}
	// Sort descending by count
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	result := make([]string, 0, topK)
	for i := 0; i < len(sorted) && i < topK; i++ {
		result = append(result, sorted[i].k)
	}
	return result, nil
}

func (s *MemTrajectoryStore) MatchedSkills(_ context.Context, projectID string, requirement string, topK int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skillCounts := make(map[string]int)
	for _, t := range s.data[projectID] {
		for _, skill := range t.SkillsMatched {
			skillCounts[skill]++
		}
	}
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range skillCounts {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	result := make([]string, 0, topK)
	for i := 0; i < len(sorted) && i < topK; i++ {
		result = append(result, sorted[i].k)
	}
	return result, nil
}
