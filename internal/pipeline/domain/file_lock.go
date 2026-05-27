package domain

import (
	"errors"
	"time"
)

var ErrFileLockConflict = errors.New("file lock conflict")

// LockType represents the type of file lock.
type LockType string

const (
	LockWrite    LockType = "write"
	LockReadOnly LockType = "read_only"
)

// FileLock represents a lock held on a file by a pipeline.
type FileLock struct {
	ID                string
	PipelineID        string
	ProjectID         string
	FilePath          string
	LockType          LockType
	EstimatedDuration int
	LockedAt          time.Time
	ExpiresAt         time.Time
}

// IsExpired checks if the lock has timed out.
func (l *FileLock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// GraphCycle holds a detected deadlock cycle in the lock dependency graph.
type GraphCycle struct {
	PipelineIDs []string
	FilePaths   []string
}

// FileLockStore persists file locks and detects deadlocks.
type FileLockStore interface {
	Acquire(pipelineID, projectID, filePath string, lockType LockType) error
	Release(projectID, filePath string) error
	ListByProject(projectID string) ([]FileLock, error)
	DetectDeadlock(projectID string) ([]GraphCycle, error)
}

// DetectCycles performs DFS-based cycle detection on a directed graph.
// adj maps node -> set of neighbor nodes. Returns all detected cycles.
// The caller is responsible for populating FilePaths on the returned cycles
// if needed, based on the graph context.
func DetectCycles(adj map[string]map[string]bool) []GraphCycle {
	var cycles []GraphCycle
	visited := make(map[string]int) // 0=unvisited, 1=in_stack, 2=done

	var dfs func(node string, stack []string)
	dfs = func(node string, stack []string) {
		if visited[node] == 1 {
			// Found a cycle — extract it from the stack.
			cycleStart := -1
			for i, n := range stack {
				if n == node {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycles = append(cycles, GraphCycle{
					PipelineIDs: stack[cycleStart:],
				})
			}
			return
		}
		if visited[node] == 2 {
			return
		}
		visited[node] = 1
		stack = append(stack, node)
		for neighbor := range adj[node] {
			dfs(neighbor, stack)
		}
		visited[node] = 2
	}

	for node := range adj {
		if visited[node] == 0 {
			dfs(node, nil)
		}
	}

	return cycles
}
