package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// KnowledgeSnapshot represents a versioned snapshot of project knowledge.
// It captures the full learning state at a point in time, enabling
// rollback and health-gated promotion across pipeline runs.
type KnowledgeSnapshot struct {
	ID                string          `json:"id"`
	ProjectID         string          `json:"project_id"`
	Version           int             `json:"version"`
	SnapshotData      json.RawMessage `json:"snapshot_data"`
	Signature         string          `json:"signature,omitempty"`
	HealthBaseline    float64         `json:"health_baseline"`
	CodeAcceptanceRate float64        `json:"code_acceptance_rate"`
	CreatedAt         time.Time       `json:"created_at"`
}

// KnowledgeSnapshotStore defines the interface for versioned knowledge
// snapshot operations.
type KnowledgeSnapshotStore interface {
	Create(ctx context.Context, snapshot *KnowledgeSnapshot) error
	GetLatest(ctx context.Context, projectID string) (*KnowledgeSnapshot, error)
	ListByProject(ctx context.Context, projectID string) ([]*KnowledgeSnapshot, error)
	Rollback(ctx context.Context, projectID string, version int) (*KnowledgeSnapshot, error)
}

// MemKnowledgeSnapshotStore is an in-memory implementation of
// KnowledgeSnapshotStore for development and testing.
type MemKnowledgeSnapshotStore struct {
	mu        sync.RWMutex
	snapshots []*KnowledgeSnapshot
	nextID    int
}

// NewMemKnowledgeSnapshotStore creates a new empty in-memory snapshot store.
func NewMemKnowledgeSnapshotStore() *MemKnowledgeSnapshotStore {
	return &MemKnowledgeSnapshotStore{
		snapshots: make([]*KnowledgeSnapshot, 0),
		nextID:    1,
	}
}

// Create stores a new knowledge snapshot, assigning unique ID and version.
func (s *MemKnowledgeSnapshotStore) Create(_ context.Context, snapshot *KnowledgeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot.ID = fmt.Sprintf("ks-%d", s.nextID)
	s.nextID++

	// Determine version: increment from latest for this project
	maxVer := 0
	for _, existing := range s.snapshots {
		if existing.ProjectID == snapshot.ProjectID && existing.Version > maxVer {
			maxVer = existing.Version
		}
	}
	snapshot.Version = maxVer + 1

	snapshot.CreatedAt = time.Now().UTC()
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}

// GetLatest returns the most recent snapshot for a project.
func (s *MemKnowledgeSnapshotStore) GetLatest(_ context.Context, projectID string) (*KnowledgeSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *KnowledgeSnapshot
	for _, snap := range s.snapshots {
		if snap.ProjectID == projectID {
			if latest == nil || snap.Version > latest.Version {
				latest = snap
			}
		}
	}
	return latest, nil
}

// ListByProject returns all snapshots for a project, ordered by version ascending.
func (s *MemKnowledgeSnapshotStore) ListByProject(_ context.Context, projectID string) ([]*KnowledgeSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*KnowledgeSnapshot
	for _, snap := range s.snapshots {
		if snap.ProjectID == projectID {
			result = append(result, snap)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

// Rollback retrieves a snapshot at a specific version, effectively
// reverting the knowledge state to that point.
func (s *MemKnowledgeSnapshotStore) Rollback(_ context.Context, projectID string, version int) (*KnowledgeSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, snap := range s.snapshots {
		if snap.ProjectID == projectID && snap.Version == version {
			return snap, nil
		}
	}
	return nil, nil
}

// compile-time interface check
var _ KnowledgeSnapshotStore = (*MemKnowledgeSnapshotStore)(nil)
