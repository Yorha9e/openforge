package service

import (
	"context"
	"time"

	"openforge/internal/pipeline/domain"
)

type FileLockService struct {
	store domain.FileLockStore
}

func NewFileLockService(store domain.FileLockStore) *FileLockService {
	return &FileLockService{store: store}
}

func (s *FileLockService) AcquireWriteLock(ctx context.Context, pipelineID, projectID, filePath string, ttl time.Duration) error {
	return s.store.Acquire(pipelineID, projectID, filePath, domain.LockWrite)
}

func (s *FileLockService) AcquireReadOnlyLock(ctx context.Context, pipelineID, projectID, filePath string, ttl time.Duration) error {
	return s.store.Acquire(pipelineID, projectID, filePath, domain.LockReadOnly)
}

func (s *FileLockService) ReleaseLock(ctx context.Context, projectID, filePath string) error {
	return s.store.Release(projectID, filePath)
}

func (s *FileLockService) ExpiredLocks(ctx context.Context, projectID string, now time.Time) ([]*domain.FileLock, error) {
	locks, err := s.store.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	var expired []*domain.FileLock
	for i := range locks {
		l := &locks[i]
		if l.ExpiresAt.Before(now) || l.ExpiresAt.Equal(now) {
			expired = append(expired, l)
		}
	}
	return expired, nil
}
