package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"openforge/internal/pipeline/domain"
)

type fakeFileLockStore struct {
	locks       map[string]*domain.FileLock
	failAcquire bool
}

func (s *fakeFileLockStore) Acquire(pipelineID, projectID, filePath string, lockType domain.LockType) error {
	if s.failAcquire {
		return errors.New("lock conflict")
	}
	key := projectID + ":" + filePath
	if _, ok := s.locks[key]; ok {
		return domain.ErrFileLockConflict
	}
	s.locks[key] = &domain.FileLock{
		PipelineID: pipelineID,
		ProjectID:  projectID,
		FilePath:   filePath,
		LockType:   lockType,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	return nil
}

func (s *fakeFileLockStore) Release(projectID, filePath string) error {
	delete(s.locks, projectID+":"+filePath)
	return nil
}

func (s *fakeFileLockStore) ListByProject(projectID string) ([]domain.FileLock, error) {
	var out []domain.FileLock
	for _, l := range s.locks {
		if l.ProjectID == projectID {
			out = append(out, *l)
		}
	}
	return out, nil
}

func (s *fakeFileLockStore) DetectDeadlock(projectID string) ([]domain.GraphCycle, error) {
	return nil, nil
}

func TestFileLockService_AcquireWriteLock(t *testing.T) {
	store := &fakeFileLockStore{locks: make(map[string]*domain.FileLock)}
	svc := NewFileLockService(store)
	err := svc.AcquireWriteLock(context.Background(), "p1", "proj1", "file.txt", 10*time.Minute)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(store.locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(store.locks))
	}
}

func TestFileLockService_ReleaseLock(t *testing.T) {
	store := &fakeFileLockStore{locks: make(map[string]*domain.FileLock)}
	svc := NewFileLockService(store)
	_ = svc.AcquireWriteLock(context.Background(), "p1", "proj1", "file.txt", 10*time.Minute)
	err := svc.ReleaseLock(context.Background(), "proj1", "file.txt")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(store.locks) != 0 {
		t.Fatalf("expected 0 locks, got %d", len(store.locks))
	}
}

func TestFileLockService_RejectsConflict(t *testing.T) {
	store := &fakeFileLockStore{locks: make(map[string]*domain.FileLock)}
	svc := NewFileLockService(store)
	_ = svc.AcquireWriteLock(context.Background(), "p1", "proj1", "file.txt", 10*time.Minute)
	err := svc.AcquireWriteLock(context.Background(), "p2", "proj1", "file.txt", 10*time.Minute)
	if !errors.Is(err, domain.ErrFileLockConflict) {
		t.Fatalf("expected ErrFileLockConflict, got %v", err)
	}
}

func TestFileLockService_ExpireTimeoutLocks(t *testing.T) {
	store := &fakeFileLockStore{locks: make(map[string]*domain.FileLock)}
	svc := NewFileLockService(store)
	_ = svc.AcquireWriteLock(context.Background(), "p1", "proj1", "file.txt", 10*time.Minute)
	
	now := time.Now().Add(11 * time.Minute)
	expired, err := svc.ExpiredLocks(context.Background(), "proj1", now)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired lock, got %d", len(expired))
	}
}
