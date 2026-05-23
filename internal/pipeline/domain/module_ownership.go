package domain

import "strings"

// ModuleOwnership maps a module to its responsible team and reviewers.
type ModuleOwnership struct {
	ProjectID        string
	ModuleName       string
	Paths            []string
	TeamName         string
	Reviewers        []string
	FallbackReviewer string
}

// OwnershipIndex indexes ownership records by file path prefix for O(1) lookup.
type OwnershipIndex struct {
	byProject map[string][]ModuleOwnership
}

// NewOwnershipIndex creates an index from ownership records.
func NewOwnershipIndex(ownerships []ModuleOwnership) *OwnershipIndex {
	idx := &OwnershipIndex{byProject: make(map[string][]ModuleOwnership)}
	for _, o := range ownerships {
		idx.byProject[o.ProjectID] = append(idx.byProject[o.ProjectID], o)
	}
	return idx
}

// FindReviewers returns reviewers responsible for the given changed files.
// Falls back to the first ownership's FallbackReviewer when no path matches.
func (idx *OwnershipIndex) FindReviewers(projectID string, changedFiles []string) []string {
	ownerships := idx.byProject[projectID]
	seen := make(map[string]bool)
	var reviewers []string
	for _, file := range changedFiles {
		for _, o := range ownerships {
			for _, prefix := range o.Paths {
				if strings.HasPrefix(file, prefix) {
					for _, r := range o.Reviewers {
						if !seen[r] {
							seen[r] = true
							reviewers = append(reviewers, r)
						}
					}
				}
			}
		}
	}
	if len(reviewers) == 0 {
		for _, o := range ownerships {
			if o.FallbackReviewer != "" {
				reviewers = append(reviewers, o.FallbackReviewer)
				break
			}
		}
	}
	return reviewers
}
