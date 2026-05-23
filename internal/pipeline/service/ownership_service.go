package service

import (
	"context"

	"openforge/internal/pipeline/domain"
)

// OwnershipRepository provides module ownership data from storage.
type OwnershipRepository interface {
	ListByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error)
}

// OwnershipService resolves reviewers based on module ownership paths.
type OwnershipService struct {
	repo OwnershipRepository
}

// NewOwnershipService creates a reviewer auto-routing service.
func NewOwnershipService(repo OwnershipRepository) *OwnershipService {
	return &OwnershipService{repo: repo}
}

// FindReviewers returns reviewers for the given changed files based on
// module ownership paths. Falls back to FallbackReviewer on no match.
func (s *OwnershipService) FindReviewers(ctx context.Context, projectID string, changedFiles []string) ([]string, error) {
	ownerships, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	idx := domain.NewOwnershipIndex(ownerships)
	return idx.FindReviewers(projectID, changedFiles), nil
}
