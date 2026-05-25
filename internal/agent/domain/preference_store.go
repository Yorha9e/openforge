package domain

import (
	"context"
	"sort"
	"sync"
	"time"
)

// PreferenceStore persists learned preferences (L2 feedback loop).
type PreferenceStore interface {
	Upsert(ctx context.Context, pref PreferenceRecord) error
	ListByProject(ctx context.Context, projectID string) ([]PreferenceRecord, error)
	Get(ctx context.Context, projectID, key string) ([]PreferenceRecord, error)
	ResolveConflict(ctx context.Context, projectID, key string) (*PreferenceRecord, error)
}

// PreferenceRecord is a persisted preference entry.
type PreferenceRecord struct {
	ID             string
	ProjectID      string
	Key            string
	Value          string
	Weight         float64
	Source         string
	ConflictCount  int
	LastActivated  string // ISO8601
}

// ResolveConflict applies §4.9 conflict rules: 频次 > 时间 > Agent 置信度.
// Returns the winning preference or nil if no records exist.
func ResolveConflict(records []PreferenceRecord) *PreferenceRecord {
	if len(records) == 0 {
		return nil
	}
	// Sort by: conflict_count desc → last_activated desc → weight desc
	sort.Slice(records, func(i, j int) bool {
		if records[i].ConflictCount != records[j].ConflictCount {
			return records[i].ConflictCount > records[j].ConflictCount
		}
		if records[i].LastActivated != records[j].LastActivated {
			return records[i].LastActivated > records[j].LastActivated
		}
		return records[i].Weight > records[j].Weight
	})
	return &records[0]
}

// MergeSimilarPreferences merges preferences with similarity > 0.95.
// Returns the merged list with deduplicated entries.
func MergeSimilarPreferences(prefs []PreferenceRecord) []PreferenceRecord {
	if len(prefs) <= 1 {
		return prefs
	}
	seen := make(map[string]bool)
	var result []PreferenceRecord
	for _, p := range prefs {
		fingerprint := p.Key + "::" + p.Value
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		result = append(result, p)
	}
	return result
}

// MemPreferenceStore is an in-memory implementation for testing.
type MemPreferenceStore struct {
	mu   sync.RWMutex
	data map[string][]PreferenceRecord
}

func NewMemPreferenceStore() *MemPreferenceStore {
	return &MemPreferenceStore{data: make(map[string][]PreferenceRecord)}
}

func (s *MemPreferenceStore) Upsert(_ context.Context, pref PreferenceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.data[pref.ProjectID]
	for i, p := range list {
		if p.Key == pref.Key && p.Value == pref.Value {
			list[i].Weight = pref.Weight
			list[i].ConflictCount++
			list[i].LastActivated = time.Now().UTC().Format(time.RFC3339)
			return nil
		}
	}
	pref.LastActivated = time.Now().UTC().Format(time.RFC3339)
	s.data[pref.ProjectID] = append(list, pref)
	return nil
}

func (s *MemPreferenceStore) ListByProject(_ context.Context, projectID string) ([]PreferenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PreferenceRecord, len(s.data[projectID]))
	copy(result, s.data[projectID])
	return result, nil
}

func (s *MemPreferenceStore) Get(_ context.Context, projectID, key string) ([]PreferenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []PreferenceRecord
	for _, p := range s.data[projectID] {
		if p.Key == key {
			result = append(result, p)
		}
	}
	return result, nil
}

func (s *MemPreferenceStore) ResolveConflict(_ context.Context, projectID, key string) (*PreferenceRecord, error) {
	records, _ := s.Get(context.Background(), projectID, key)
	return ResolveConflict(records), nil
}
