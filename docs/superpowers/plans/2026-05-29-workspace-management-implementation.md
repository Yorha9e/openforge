# Workspace Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement project-level workspace management with Git integration, replacing the current global localStorage approach.

**Architecture:** Database-driven + Filesystem abstraction (方案 C). Each project gets its own workspace path with full Git integration via server-side WorkspaceManager and GitExecutor services.

**Tech Stack:** Go (backend), PostgreSQL (metadata), React + TypeScript (frontend), Git CLI (version control), WebSocket (real-time updates)

---

## File Structure

### Backend Files

- `migrations/002_workspace.up.sql` - Database migration for workspace fields and table
- `internal/workspace/domain/workspace.go` - Workspace domain models and interfaces
- `internal/workspace/domain/errors.go` - Workspace-specific error types
- `internal/workspace/domain/file_tree.go` - File tree data structures
- `internal/workspace/port/repository.go` - Workspace repository interface
- `internal/workspace/adapter/pg_workspace_repository.go` - PostgreSQL workspace repository
- `internal/workspace/adapter/git_executor.go` - Git CLI wrapper
- `internal/workspace/service/workspace_manager.go` - Core workspace management service
- `internal/workspace/service/file_tree_service.go` - File tree building service
- `internal/server/workspace_handler.go` - HTTP handlers for workspace API

### Frontend Files

- `frontend/src/shared/api.ts` - Add workspace API types and functions
- `frontend/src/features/workspace/GitPanel.tsx` - Main Git panel component
- `frontend/src/features/workspace/GitStatus.tsx` - Git status display
- `frontend/src/features/workspace/GitToolbar.tsx` - Git action toolbar
- `frontend/src/features/workspace/GitCommitInput.tsx` - Commit message input
- `frontend/src/features/workspace/GitDiffView.tsx` - Diff viewer component
- `frontend/src/features/workspace/GitHistory.tsx` - Commit history timeline
- `frontend/src/features/workspace/BranchManager.tsx` - Branch management panel
- `frontend/src/features/workspace/FileTreePanel.tsx` - Enhanced file tree (Phase 2.5)
- `frontend/src/features/workspace/workspace-context.tsx` - Workspace state management

### Test Files

- `internal/workspace/service/workspace_manager_test.go` - WorkspaceManager unit tests
- `internal/workspace/adapter/git_executor_test.go` - GitExecutor unit tests
- `internal/workspace/service/file_tree_service_test.go` - File tree service tests
- `frontend/src/features/workspace/__tests__/GitPanel.test.tsx` - GitPanel component tests

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/002_workspace.up.sql`

- [ ] **Step 1: Create migration file**

```sql
-- 002_workspace.up.sql — Workspace management tables

-- 1. Extend project table with workspace fields
ALTER TABLE project ADD COLUMN workspace_path VARCHAR(512) DEFAULT '';
ALTER TABLE project ADD COLUMN git_remote_url VARCHAR(512) DEFAULT '';
ALTER TABLE project ADD COLUMN git_branch VARCHAR(64) DEFAULT 'main';

-- 2. Create workspace_config table
CREATE TABLE IF NOT EXISTS workspace_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    root_path VARCHAR(512) NOT NULL,
    auto_push_interval INT DEFAULT 30,
    auto_stage BOOLEAN DEFAULT true,
    auto_push BOOLEAN DEFAULT false,
    max_workspace_size_mb INT DEFAULT 5000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Insert default configuration
INSERT INTO workspace_config (root_path, auto_push_interval, auto_stage, auto_push)
VALUES ('/data/openforge/workspace', 30, true, false);

-- 4. Create index for workspace queries
CREATE INDEX IF NOT EXISTS idx_project_workspace_path ON project(workspace_path) WHERE workspace_path != '';
```

- [ ] **Step 2: Run migration**

Run: `psql -h localhost -p 5432 -U openforge -d openforge -f migrations/002_workspace.up.sql`

Expected: Migration completes successfully without errors.

- [ ] **Step 3: Verify migration**

Run: `psql -h localhost -p 5432 -U openforge -d openforge -c "\d project"`

Expected: Output shows new columns `workspace_path`, `git_remote_url`, `git_branch`.

Run: `psql -h localhost -p 5432 -U openforge -d openforge -c "\d workspace_config"`

Expected: Output shows `workspace_config` table structure.

- [ ] **Step 4: Commit**

```bash
git add migrations/002_workspace.up.sql
git commit -m "feat: add workspace management database schema"
```

---

## Task 2: Workspace Domain Models

**Files:**
- Create: `internal/workspace/domain/workspace.go`
- Create: `internal/workspace/domain/errors.go`

- [ ] **Step 1: Create workspace domain models**

```go
// internal/workspace/domain/workspace.go
package domain

import (
	"time"
)

// WorkspaceConfig represents the global workspace configuration
type WorkspaceConfig struct {
	ID                 string `json:"id"`
	RootPath           string `json:"root_path"`
	AutoPushInterval   int    `json:"auto_push_interval"`
	AutoStage          bool   `json:"auto_stage"`
	AutoPush           bool   `json:"auto_push"`
	MaxWorkspaceSizeMB int    `json:"max_workspace_size_mb"`
}

// Workspace represents a project's workspace
type Workspace struct {
	ProjectID   string    `json:"project_id"`
	Path        string    `json:"path"`
	GitRemote   string    `json:"git_remote"`
	GitBranch   string    `json:"git_branch"`
	LastSync    time.Time `json:"last_sync"`
	Initialized bool      `json:"initialized"`
}

// WorkspaceStatus represents the current Git status of a workspace
type WorkspaceStatus struct {
	ProjectID      string   `json:"project_id"`
	Path           string   `json:"path"`
	Branch         string   `json:"branch"`
	ModifiedFiles  []string `json:"modified_files"`
	StagedFiles    []string `json:"staged_files"`
	UntrackedFiles []string `json:"untracked_files"`
	LastCommit     string   `json:"last_commit"`
	LastPush       string   `json:"last_push"`
	IsClean        bool     `json:"is_clean"`
	RemoteURL      string   `json:"remote_url"`
	HasRemote      bool     `json:"has_remote"`
}

// GitCommit represents a Git commit
type GitCommit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

// GitBranch represents a Git branch
type GitBranch struct {
	Name       string `json:"name"`
	IsCurrent  bool   `json:"is_current"`
	IsRemote   bool   `json:"is_remote"`
	LastCommit string `json:"last_commit"`
}
```

- [ ] **Step 2: Create error types**

```go
// internal/workspace/domain/errors.go
package domain

import "errors"

var (
	// ErrWorkspaceNotFound indicates the workspace does not exist
	ErrWorkspaceNotFound = errors.New("workspace not found")
	
	// ErrWorkspaceExists indicates the workspace is already initialized
	ErrWorkspaceExists = errors.New("workspace already initialized")
	
	// ErrGitNotInstalled indicates Git is not available
	ErrGitNotInstalled = errors.New("git is not installed")
	
	// ErrRemoteNotConfigured indicates no remote repository is configured
	ErrRemoteNotConfigured = errors.New("remote repository not configured")
	
	// ErrConflictDetected indicates a merge conflict
	ErrConflictDetected = errors.New("merge conflict detected")
	
	// ErrPushRejected indicates push was rejected (need to pull first)
	ErrPushRejected = errors.New("push rejected")
	
	// ErrNetworkError indicates a network error
	ErrNetworkError = errors.New("network error")
	
	// ErrPermissionDenied indicates permission denied
	ErrPermissionDenied = errors.New("permission denied")
	
	// ErrWorkspaceSizeExceeded indicates workspace size limit exceeded
	ErrWorkspaceSizeExceeded = errors.New("workspace size limit exceeded")
	
	// ErrInvalidPath indicates an invalid workspace path
	ErrInvalidPath = errors.New("invalid workspace path")
)
```

- [ ] **Step 3: Verify compilation**

Run: `cd internal/workspace/domain && go build .`

Expected: No compilation errors.

- [ ] **Step 4: Commit**

```bash
git add internal/workspace/domain/
git commit -m "feat: add workspace domain models and error types"
```

---

## Task 3: Workspace Repository Interface

**Files:**
- Create: `internal/workspace/port/repository.go`

- [ ] **Step 1: Create repository interface**

```go
// internal/workspace/port/repository.go
package port

import (
	"context"
	
	"openforge/internal/workspace/domain"
)

// WorkspaceRepository defines the interface for workspace persistence
type WorkspaceRepository interface {
	// GetConfig retrieves the global workspace configuration
	GetConfig(ctx context.Context) (*domain.WorkspaceConfig, error)
	
	// UpdateConfig updates the global workspace configuration
	UpdateConfig(ctx context.Context, config *domain.WorkspaceConfig) error
	
	// GetWorkspace retrieves a workspace by project ID
	GetWorkspace(ctx context.Context, projectID string) (*domain.Workspace, error)
	
	// CreateWorkspace creates a new workspace
	CreateWorkspace(ctx context.Context, ws *domain.Workspace) error
	
	// UpdateWorkspace updates an existing workspace
	UpdateWorkspace(ctx context.Context, ws *domain.Workspace) error
	
	// DeleteWorkspace removes a workspace
	DeleteWorkspace(ctx context.Context, projectID) error
	
	// UpdateProjectWorkspace updates the project table with workspace info
	UpdateProjectWorkspace(ctx context.Context, projectID, workspacePath, gitRemote, gitBranch string) error
	
	// GetProjectWorkspace retrieves workspace info from project table
	GetProjectWorkspace(ctx context.Context, projectID string) (workspacePath, gitRemote, gitBranch string, err error)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd internal/workspace/port && go build .`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/workspace/port/
git commit -m "feat: add workspace repository interface"
```

---

## Task 4: PostgreSQL Workspace Repository

**Files:**
- Create: `internal/workspace/adapter/pg_workspace_repository.go`

- [ ] **Step 1: Create PostgreSQL repository implementation**

```go
// internal/workspace/adapter/pg_workspace_repository.go
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	
	"openforge/internal/workspace/domain"
	"openforge/internal/workspace/port"
)

var _ port.WorkspaceRepository = (*PGWorkspaceRepository)(nil)

type PGWorkspaceRepository struct {
	db *sql.DB
}

func NewPGWorkspaceRepository(db *sql.DB) *PGWorkspaceRepository {
	return &PGWorkspaceRepository{db: db}
}

func (r *PGWorkspaceRepository) GetConfig(ctx context.Context) (*domain.WorkspaceConfig, error) {
	var config domain.WorkspaceConfig
	err := r.db.QueryRowContext(ctx, `
		SELECT id, root_path, auto_push_interval, auto_stage, auto_push, max_workspace_size_mb
		FROM workspace_config
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(
		&config.ID, &config.RootPath, &config.AutoPushInterval,
		&config.AutoStage, &config.AutoPush, &config.MaxWorkspaceSizeMB,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workspace config not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace config: %w", err)
	}
	return &config, nil
}

func (r *PGWorkspaceRepository) UpdateConfig(ctx context.Context, config *domain.WorkspaceConfig) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE workspace_config 
		SET root_path = $1, auto_push_interval = $2, auto_stage = $3, 
		    auto_push = $4, max_workspace_size_mb = $5, updated_at = NOW()
		WHERE id = $6
	`, config.RootPath, config.AutoPushInterval, config.AutoStage,
		config.AutoPush, config.MaxWorkspaceSizeMB, config.ID)
	return err
}

func (r *PGWorkspaceRepository) GetWorkspace(ctx context.Context, projectID string) (*domain.Workspace, error) {
	var ws domain.Workspace
	var gitRemote, gitBranch sql.NullString
	
	err := r.db.QueryRowContext(ctx, `
		SELECT workspace_path, git_remote_url, git_branch
		FROM project
		WHERE id = $1 AND deleted_at IS NULL
	`, projectID).Scan(&ws.Path, &gitRemote, &gitBranch)
	
	if err == sql.ErrNoRows {
		return nil, domain.ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	
	ws.ProjectID = projectID
	if gitRemote.Valid {
		ws.GitRemote = gitRemote.String
	}
	if gitBranch.Valid {
		ws.GitBranch = gitBranch.String
	}
	ws.Initialized = ws.Path != ""
	
	return &ws, nil
}

func (r *PGWorkspaceRepository) CreateWorkspace(ctx context.Context, ws *domain.Workspace) error {
	return r.UpdateProjectWorkspace(ctx, ws.ProjectID, ws.Path, ws.GitRemote, ws.GitBranch)
}

func (r *PGWorkspaceRepository) UpdateWorkspace(ctx context.Context, ws *domain.Workspace) error {
	return r.UpdateProjectWorkspace(ctx, ws.ProjectID, ws.Path, ws.GitRemote, ws.GitBranch)
}

func (r *PGWorkspaceRepository) DeleteWorkspace(ctx context.Context, projectID string) error {
	return r.UpdateProjectWorkspace(ctx, projectID, "", "", "main")
}

func (r *PGWorkspaceRepository) UpdateProjectWorkspace(ctx context.Context, projectID, workspacePath, gitRemote, gitBranch string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE project 
		SET workspace_path = $1, git_remote_url = $2, git_branch = $3, updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
	`, workspacePath, gitRemote, gitBranch, projectID)
	return err
}

func (r *PGWorkspaceRepository) GetProjectWorkspace(ctx context.Context, projectID string) (workspacePath, gitRemote, gitBranch string, err error) {
	var wp, gr sql.NullString
	var gb string
	
	err = r.db.QueryRowContext(ctx, `
		SELECT workspace_path, git_remote_url, git_branch
		FROM project
		WHERE id = $1 AND deleted_at IS NULL
	`, projectID).Scan(&wp, &gr, &gb)
	
	if err == sql.ErrNoRows {
		return "", "", "", domain.ErrWorkspaceNotFound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("get project workspace: %w", err)
	}
	
	if wp.Valid {
		workspacePath = wp.String
	}
	if gr.Valid {
		gitRemote = gr.String
	}
	gitBranch = gb
	
	return workspacePath, gitRemote, gitBranch, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd internal/workspace/adapter && go build .`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/workspace/adapter/pg_workspace_repository.go
git commit -m "feat: add PostgreSQL workspace repository implementation"
```

---

## Task 5: Git Executor

**Files:**
- Create: `internal/workspace/adapter/git_executor.go`

- [ ] **Step 1: Create Git executor implementation**

```go
// internal/workspace/adapter/git_executor.go
package adapter

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	
	"openforge/internal/workspace/domain"
)

// GitExecutor wraps Git CLI operations
type GitExecutor struct {
	workDir string
}

// NewGitExecutor creates a new GitExecutor for the given working directory
func NewGitExecutor(workDir string) *GitExecutor {
	return &GitExecutor{workDir: workDir}
}

// Run executes a Git command and returns the output
func (ge *GitExecutor) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = ge.workDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %w\nOutput: %s", 
			strings.Join(args, " "), err, string(output))
	}
	
	return strings.TrimSpace(string(output)), nil
}

// Init initializes a new Git repository
func (ge *GitExecutor) Init(bare bool) error {
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}
	_, err := ge.Run(args...)
	return err
}

// AddRemote adds a remote repository
func (ge *GitExecutor) AddRemote(name, url string) error {
	_, err := ge.Run("remote", "add", name, url)
	return err
}

// Fetch fetches from a remote
func (ge *GitExecutor) Fetch(remote string) error {
	_, err := ge.Run("fetch", remote)
	return err
}

// Checkout switches to a branch
func (ge *GitExecutor) Checkout(branch string) error {
	_, err := ge.Run("checkout", branch)
	return err
}

// CreateBranch creates a new branch
func (ge *GitExecutor) CreateBranch(branch string) error {
	_, err := ge.Run("branch", branch)
	return err
}

// Add stages files
func (ge *GitExecutor) Add(files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := ge.Run(args...)
	return err
}

// Commit creates a commit
func (ge *GitExecutor) Commit(message string) error {
	_, err := ge.Run("commit", "-m", message)
	return err
}

// Push pushes to a remote branch
func (ge *GitExecutor) Push(remote, branch string, force bool) error {
	args := []string{"push", remote, branch}
	if force {
		args = append(args, "--force")
	}
	_, err := ge.Run(args...)
	return err
}

// Pull pulls from a remote branch
func (ge *GitExecutor) Pull(remote, branch string) error {
	_, err := ge.Run("pull", remote, branch)
	return err
}

// Status returns the current Git status
func (ge *GitExecutor) Status() (*domain.WorkspaceStatus, error) {
	// Get current branch
	branch, err := ge.Run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}
	
	// Get status porcelain
	statusOutput, err := ge.Run("status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}
	
	// Parse status
	var modified, staged, untracked []string
	if statusOutput != "" {
		lines := strings.Split(statusOutput, "\n")
		for _, line := range lines {
			if len(line) < 3 {
				continue
			}
			statusCode := line[:2]
			filePath := strings.TrimSpace(line[3:])
			
			if statusCode[0] != ' ' && statusCode[0] != '?' {
				staged = append(staged, filePath)
			}
			if statusCode[1] == 'M' || statusCode[1] == 'D' {
				modified = append(modified, filePath)
			}
			if statusCode == "??" {
				untracked = append(untracked, filePath)
			}
		}
	}
	
	// Get last commit
	lastCommit, _ := ge.Run("log", "-1", "--pretty=format:%H", "2>/dev/null")
	
	// Get remote URL
	remoteURL, _ := ge.Run("remote", "get-url", "origin", "2>/dev/null")
	
	return &domain.WorkspaceStatus{
		Branch:         branch,
		ModifiedFiles:  modified,
		StagedFiles:    staged,
		UntrackedFiles: untracked,
		LastCommit:     lastCommit,
		IsClean:        len(modified) == 0 && len(staged) == 0 && len(untracked) == 0,
		RemoteURL:      remoteURL,
		HasRemote:      remoteURL != "",
	}, nil
}

// Diff returns the diff for a specific file
func (ge *GitExecutor) Diff(file string) (string, error) {
	return ge.Run("diff", file)
}

// Log returns commit history
func (ge *GitExecutor) Log(limit int) ([]domain.GitCommit, error) {
	output, err := ge.Run("log", fmt.Sprintf("-%d", limit), 
		"--pretty=format:%H|%s|%an|%ae|%aI")
	if err != nil {
		return nil, err
	}
	
	if output == "" {
		return []domain.GitCommit{}, nil
	}
	
	var commits []domain.GitCommit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}
		
		timestamp, _ := time.Parse(time.RFC3339, parts[4])
		commits = append(commits, domain.GitCommit{
			Hash:      parts[0],
			Message:   parts[1],
			Author:    parts[2],
			Email:     parts[3],
			Timestamp: timestamp,
		})
	}
	
	return commits, nil
}

// Merge merges a branch with the specified strategy
func (ge *GitExecutor) Merge(branch string, strategy string) error {
	args := []string{"merge"}
	switch strategy {
	case "ours":
		args = append(args, "-X", "ours")
	case "theirs":
		args = append(args, "-X", "theirs")
	}
	args = append(args, branch)
	_, err := ge.Run(args...)
	return err
}

// GetBranches returns all branches
func (ge *GitExecutor) GetBranches() ([]domain.GitBranch, error) {
	output, err := ge.Run("branch", "-a", "--format=%(refname:short)|%(HEAD)|%(objectname:short)")
	if err != nil {
		return nil, err
	}
	
	var branches []domain.GitBranch
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		
		isCurrent := parts[1] == "*"
		isRemote := strings.HasPrefix(parts[0], "origin/")
		
		branches = append(branches, domain.GitBranch{
			Name:      parts[0],
			IsCurrent: isCurrent,
			IsRemote:  isRemote,
			LastCommit: parts[2],
		})
	}
	
	return branches, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd internal/workspace/adapter && go build .`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/workspace/adapter/git_executor.go
git commit -m "feat: add Git executor for workspace operations"
```

---

## Task 6: Workspace Manager Service

**Files:**
- Create: `internal/workspace/service/workspace_manager.go`

- [ ] **Step 1: Create WorkspaceManager service**

```go
// internal/workspace/service/workspace_manager.go
package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"openforge/internal/workspace/adapter"
	"openforge/internal/workspace/domain"
	"openforge/internal/workspace/port"
)

// WorkspaceManager manages project workspaces
type WorkspaceManager struct {
	repo       port.WorkspaceRepository
	config     *domain.WorkspaceConfig
	mu         sync.RWMutex
	workspaces map[string]*workspaceEntry
}

type workspaceEntry struct {
	workspace *domain.Workspace
	git       *adapter.GitExecutor
	lastSync  time.Time
}

// NewWorkspaceManager creates a new WorkspaceManager
func NewWorkspaceManager(repo port.WorkspaceRepository) (*WorkspaceManager, error) {
	config, err := repo.GetConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get workspace config: %w", err)
	}
	
	return &WorkspaceManager{
		repo:       repo,
		config:     config,
		workspaces: make(map[string]*workspaceEntry),
	}, nil
}

// InitWorkspace initializes a workspace for a project
func (wm *WorkspaceManager) InitWorkspace(ctx context.Context, projectID, gitRemoteURL string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	// Check if already initialized
	existing, err := wm.repo.GetWorkspace(ctx, projectID)
	if err != nil && err != domain.ErrWorkspaceNotFound {
		return fmt.Errorf("check existing workspace: %w", err)
	}
	if existing != nil && existing.Initialized {
		return domain.ErrWorkspaceExists
	}
	
	// Create workspace directory
	wsPath := filepath.Join(wm.config.RootPath, projectID)
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	
	// Initialize Git repository
	git := adapter.NewGitExecutor(wsPath)
	if err := git.Init(false); err != nil {
		return fmt.Errorf("init git repository: %w", err)
	}
	
	// Add remote if provided
	if gitRemoteURL != "" {
		if err := git.AddRemote("origin", gitRemoteURL); err != nil {
			return fmt.Errorf("add remote: %w", err)
		}
		
		// Try to pull initial content
		if err := git.Fetch("origin"); err != nil {
			slog.Warn("failed to fetch from remote", "error", err)
		}
	}
	
	// Save workspace info
	ws := &domain.Workspace{
		ProjectID:   projectID,
		Path:        wsPath,
		GitRemote:   gitRemoteURL,
		GitBranch:   "main",
		Initialized: true,
	}
	
	if err := wm.repo.CreateWorkspace(ctx, ws); err != nil {
		return fmt.Errorf("save workspace: %w", err)
	}
	
	// Cache workspace
	wm.workspaces[projectID] = &workspaceEntry{
		workspace: ws,
		git:       git,
		lastSync:  time.Now(),
	}
	
	slog.Info("workspace initialized",
		"project_id", projectID,
		"path", wsPath,
		"has_remote", gitRemoteURL != "",
	)
	
	return nil
}

// getWorkspace returns the workspace entry for a project
func (wm *WorkspaceManager) getWorkspace(projectID string) (*workspaceEntry, error) {
	// Check cache first
	if entry, ok := wm.workspaces[projectID]; ok {
		return entry, nil
	}
	
	// Load from database
	ws, err := wm.repo.GetWorkspace(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	
	if !ws.Initialized {
		return nil, domain.ErrWorkspaceNotFound
	}
	
	// Create Git executor
	git := adapter.NewGitExecutor(ws.Path)
	
	entry := &workspaceEntry{
		workspace: ws,
		git:       git,
		lastSync:  time.Now(),
	}
	
	// Cache it
	wm.workspaces[projectID] = entry
	
	return entry, nil
}

// GetStatus returns the Git status of a workspace
func (wm *WorkspaceManager) GetStatus(ctx context.Context, projectID string) (*domain.WorkspaceStatus, error) {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	status, err := entry.git.Status()
	if err != nil {
		return nil, fmt.Errorf("get git status: %w", err)
	}
	
	status.ProjectID = projectID
	status.Path = entry.workspace.Path
	
	return status, nil
}

// StageFiles stages files for commit
func (wm *WorkspaceManager) StageFiles(ctx context.Context, projectID string, files []string) error {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	return entry.git.Add(files...)
}

// Commit creates a commit with the given message
func (wm *WorkspaceManager) Commit(ctx context.Context, projectID, message string) error {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	return entry.git.Commit(message)
}

// Push pushes changes to the remote repository
func (wm *WorkspaceManager) Push(ctx context.Context, projectID string) error {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	if entry.workspace.GitRemote == "" {
		return domain.ErrRemoteNotConfigured
	}
	
	branch := entry.workspace.GitBranch
	if branch == "" {
		branch = "main"
	}
	
	if err := entry.git.Push("origin", branch, false); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrPushRejected, err)
	}
	
	return nil
}

// Pull pulls changes from the remote repository
func (wm *WorkspaceManager) Pull(ctx context.Context, projectID string) error {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	if entry.workspace.GitRemote == "" {
		return domain.ErrRemoteNotConfigured
	}
	
	branch := entry.workspace.GitBranch
	if branch == "" {
		branch = "main"
	}
	
	if err := entry.git.Pull("origin", branch); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	
	return nil
}

// GetDiff returns the diff for a specific file
func (wm *WorkspaceManager) GetDiff(ctx context.Context, projectID, file string) (string, error) {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	return entry.git.Diff(file)
}

// GetLog returns commit history
func (wm *WorkspaceManager) GetLog(ctx context.Context, projectID string, limit int) ([]domain.GitCommit, error) {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	return entry.git.Log(limit)
}

// GetBranches returns all branches
func (wm *WorkspaceManager) GetBranches(ctx context.Context, projectID string) ([]domain.GitBranch, error) {
	entry, err := wm.getWorkspace(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrWorkspaceNotFound, err)
	}
	
	return entry.git.GetBranches()
}

// Cleanup removes a workspace
func (wm *WorkspaceManager) Cleanup(ctx context.Context, projectID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	// Remove from cache
	delete(wm.workspaces, projectID)
	
	// Update database
	if err := wm.repo.DeleteWorkspace(ctx, projectID); err != nil {
		return fmt.Errorf("delete workspace from db: %w", err)
	}
	
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd internal/workspace/service && go build .`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/workspace/service/workspace_manager.go
git commit -m "feat: add WorkspaceManager service implementation"
```

---

## Task 7: File Tree Service

**Files:**
- Create: `internal/workspace/domain/file_tree.go`
- Create: `internal/workspace/service/file_tree_service.go`

- [ ] **Step 1: Create file tree domain model**

```go
// internal/workspace/domain/file_tree.go
package domain

import "time"

// FileTreeNode represents a node in the file tree
type FileTreeNode struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Type        string          `json:"type"` // "file" or "directory"
	Size        int64           `json:"size,omitempty"`
	Modified    time.Time       `json:"modified,omitempty"`
	GitStatus   string          `json:"git_status,omitempty"` // added, modified, deleted
	Children    []*FileTreeNode `json:"children,omitempty"`
	TotalFiles  int             `json:"total_files,omitempty"`
	TotalDirs   int             `json:"total_dirs,omitempty"`
}

// FileTreeResponse represents the API response for file tree
type FileTreeResponse struct {
	Root string        `json:"root"`
	Path string        `json:"path"`
	Tree *FileTreeNode `json:"tree"`
}
```

- [ ] **Step 2: Create file tree service**

```go
// internal/workspace/service/file_tree_service.go
package service

import (
	"os"
	"path/filepath"
	"strings"
	
	"openforge/internal/workspace/domain"
)

// FileTreeService builds file trees for workspaces
type FileTreeService struct{}

// NewFileTreeService creates a new FileTreeService
func NewFileTreeService() *FileTreeService {
	return &FileTreeService{}
}

// BuildFileTree builds a file tree for the given path
func (fts *FileTreeService) BuildFileTree(fullPath, relPath string, depth, maxDepth int) (*domain.FileTreeNode, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	
	node := &domain.FileTreeNode{
		Name:     info.Name(),
		Path:     relPath,
		Type:     "file",
		Size:     info.Size(),
		Modified: info.ModTime(),
	}
	
	if info.IsDir() {
		node.Type = "directory"
		node.Children = []*domain.FileTreeNode{}
		
		if depth < maxDepth {
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return nil, err
			}
			
			for _, entry := range entries {
				childPath := filepath.Join(relPath, entry.Name())
				childFullPath := filepath.Join(fullPath, entry.Name())
				
				// Skip hidden files and common ignore patterns
				if fts.shouldIgnore(entry.Name()) {
					continue
				}
				
				child, err := fts.BuildFileTree(childFullPath, childPath, depth+1, maxDepth)
				if err != nil {
					continue // Skip files we can't read
				}
				
				node.Children = append(node.Children, child)
				if child.Type == "directory" {
					node.TotalDirs += 1 + child.TotalDirs
					node.TotalFiles += child.TotalFiles
				} else {
					node.TotalFiles++
				}
			}
		}
	}
	
	return node, nil
}

// shouldIgnore checks if a file/directory should be ignored
func (fts *FileTreeService) shouldIgnore(name string) bool {
	// Ignore hidden files, node_modules, .git, etc.
	ignorePatterns := []string{".git", "node_modules", ".DS_Store", "__pycache__", ".venv", "venv"}
	for _, pattern := range ignorePatterns {
		if name == pattern {
			return true
		}
	}
	return strings.HasPrefix(name, ".")
}

// ApplyGitStatus applies Git status to the file tree
func (fts *FileTreeService) ApplyGitStatus(node *domain.FileTreeNode, gitStatus *domain.WorkspaceStatus) {
	if gitStatus == nil {
		return
	
	// Create maps for quick lookup
	modifiedMap := make(map[string]bool)
	stagedMap := make(map[string]bool)
	untrackedMap := make(map[string]bool)
	
	for _, f := range gitStatus.ModifiedFiles {
		modifiedMap[f] = true
	}
	for _, f := range gitStatus.StagedFiles {
		stagedMap[f] = true
	}
	for _, f := range gitStatus.UntrackedFiles {
		untrackedMap[f] = true
	}
	
	// Apply status recursively
	fts.applyStatusRecursive(node, modifiedMap, stagedMap, untrackedMap)
}

func (fts *FileTreeService) applyStatusRecursive(
	node *domain.FileTreeNode,
	modified, staged, untracked map[string]bool,
) {
	if node.Type == "file" {
		if staged[node.Path] {
			node.GitStatus = "added"
		} else if modified[node.Path] {
			node.GitStatus = "modified"
		} else if untracked[node.Path] {
			node.GitStatus = "untracked"
		}
	}
	
	for _, child := range node.Children {
		fts.applyStatusRecursive(child, modified, staged, untracked)
	}
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd internal/workspace/service && go build .`

Expected: No compilation errors.

- [ ] **Step 4: Commit**

```bash
git add internal/workspace/domain/file_tree.go internal/workspace/service/file_tree_service.go
git commit -m "feat: add file tree service for workspace browsing"
```

---

## Task 8: Workspace HTTP Handlers

**Files:**
- Create: `internal/server/workspace_handler.go`

- [ ] **Step 1: Create workspace HTTP handlers**

```go
// internal/server/workspace_handler.go
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	
	"openforge/internal/workspace/domain"
	"openforge/internal/workspace/service"
)

// WorkspaceHandler handles workspace-related HTTP requests
type WorkspaceHandler struct {
	wm         *service.WorkspaceManager
	fileTree   *service.FileTreeService
}

// NewWorkspaceHandler creates a new WorkspaceHandler
func NewWorkspaceHandler(wm *service.WorkspaceManager, fileTree *service.FileTreeService) *WorkspaceHandler {
	return &WorkspaceHandler{
		wm:       wm,
		fileTree: fileTree,
	}
}

// InitWorkspace handles POST /api/projects/{id}/workspace/init
func (h *WorkspaceHandler) InitWorkspace(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	var req struct {
		GitRemoteURL string `json:"git_remote_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	
	}
	
	if err := h.wm.InitWorkspace(r.Context(), projectID, req.GitRemoteURL); err != nil {
		if err == domain.ErrWorkspaceExists {
			writeError(w, 409, "workspace already initialized")
			return
		}
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "initialized"})
}

// GetStatus handles GET /api/projects/{id}/workspace/status
func (h *WorkspaceHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	status, err := h.wm.GetStatus(r.Context(), projectID)
	if err != nil {
		if err == domain.ErrWorkspaceNotFound {
			writeError(w, 404, "workspace not found")
			return
		}
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, status)
}

// StageFiles handles POST /api/projects/{id}/workspace/stage
func (h *WorkspaceHandler) StageFiles(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	var req struct {
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	
	if err := h.wm.StageFiles(r.Context(), projectID, req.Files); err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "staged"})
}

// Commit handles POST /api/projects/{id}/workspace/commit
func (h *WorkspaceHandler) Commit(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	
	if req.Message == "" {
		writeError(w, 400, "commit message required")
		return
	}
	
	if err := h.wm.Commit(r.Context(), projectID, req.Message); err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "committed"})
}

// Push handles POST /api/projects/{id}/workspace/push
func (h *WorkspaceHandler) Push(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	if err := h.wm.Push(r.Context(), projectID); err != nil {
		if err == domain.ErrRemoteNotConfigured {
			writeError(w, 400, "remote repository not configured")
			return
		}
		if err == domain.ErrPushRejected {
			writeError(w, 409, "push rejected, pull first")
			return
		}
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "pushed"})
}

// Pull handles POST /api/projects/{id}/workspace/pull
func (h *WorkspaceHandler) Pull(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	if err := h.wm.Pull(r.Context(), projectID); err != nil {
		if err == domain.ErrRemoteNotConfigured {
			writeError(w, 400, "remote repository not configured")
			return
		}
		if err == domain.ErrConflictDetected {
			writeError(w, 409, "merge conflict detected")
			return
		}
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "pulled"})
}

// GetDiff handles GET /api/projects/{id}/workspace/diff
func (h *WorkspaceHandler) GetDiff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	file := r.URL.Query().Get("file")
	
	if file == "" {
		writeError(w, 400, "file parameter required")
		return
	}
	
	diff, err := h.wm.GetDiff(r.Context(), projectID, file)
	if err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, map[string]string{"diff": diff})
}

// GetLog handles GET /api/projects/{id}/workspace/log
func (h *WorkspaceHandler) GetLog(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	limit := 50
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	
	commits, err := h.wm.GetLog(r.Context(), projectID, limit)
	if err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, commits)
}

// GetBranches handles GET /api/projects/{id}/workspace/branches
func (h *WorkspaceHandler) GetBranches(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	
	branches, err := h.wm.GetBranches(r.Context(), projectID)
	if err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	writeJSON(w, 200, branches)
}

// GetFileTree handles GET /api/projects/{id}/workspace/tree
func (h *WorkspaceHandler) GetFileTree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	subPath := r.URL.Query().Get("path")
	depth := 3
	
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 10 {
			depth = parsed
		}
	}
	
	// Get workspace path
	status, err := h.wm.GetStatus(r.Context(), projectID)
	if err != nil {
		writeError(w, 404, "workspace not found")
		return
	}
	
	fullPath := status.Path
	if subPath != "" {
		fullPath = fullPath + "/" + subPath
	}
	
	// Build file tree
	tree, err := h.fileTree.BuildFileTree(fullPath, subPath, 0, depth)
	if err != nil {
		writeError(w, 500, sanitizeError(err))
		return
	}
	
	// Apply Git status
	h.fileTree.ApplyGitStatus(tree, status)
	
	writeJSON(w, 200, domain.FileTreeResponse{
		Root: status.Path,
		Path: subPath,
		Tree: tree,
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd internal/server && go build .`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/workspace_handler.go
git commit -m "feat: add workspace HTTP handlers"
```

---

## Task 9: Register Workspace Routes

**Files:**
- Modify: `internal/server/routes.go`
- Modify: `internal/shared/profile/bootstrap.go`

- [ ] **Step 1: Add workspace initialization to bootstrap**

Add to `internal/shared/profile/bootstrap.go` after pipeline service initialization:

```go
// Workspace management
workspaceRepo := workspaceadapter.NewPGWorkspaceRepository(db)
wm, err := service.NewWorkspaceManager(workspaceRepo)
if err != nil {
    return nil, fmt.Errorf("workspace manager: %w", err)
}
fileTreeSvc := service.NewFileTreeService()
of.WorkspaceManager = wm
of.FileTreeService = fileTreeSvc
```

Also add imports:
```go
import (
    workspaceadapter "openforge/internal/workspace/adapter"
    workspaceservice "openforge/internal/workspace/service"
)
```

And add fields to OpenForge struct:
```go
WorkspaceManager *workspaceservice.WorkspaceManager
FileTreeService  *workspaceservice.FileTreeService
```

- [ ] **Step 2: Register workspace routes**

Add to `internal/server/routes.go` in the `RegisterRoutes` function:

```go
// Workspace management
workspaceHandler := NewWorkspaceHandler(of.WorkspaceManager, of.FileTreeService)
mux.HandleFunc("POST /api/projects/{id}/workspace/init", withRole("pm", workspaceHandler.InitWorkspace))
mux.HandleFunc("GET /api/projects/{id}/workspace/status", withRole("observer", workspaceHandler.GetStatus))
mux.HandleFunc("POST /api/projects/{id}/workspace/stage", withRole("pm", workspaceHandler.StageFiles))
mux.HandleFunc("POST /api/projects/{id}/workspace/commit", withRole("pm", workspaceHandler.Commit))
mux.HandleFunc("POST /api/projects/{id}/workspace/push", withRole("pm", workspaceHandler.Push))
mux.HandleFunc("POST /api/projects/{id}/workspace/pull", withRole("pm", workspaceHandler.Pull))
mux.HandleFunc("GET /api/projects/{id}/workspace/diff", withRole("observer", workspaceHandler.GetDiff))
mux.HandleFunc("GET /api/projects/{id}/workspace/log", withRole("observer", workspaceHandler.GetLog))
mux.HandleFunc("GET /api/projects/{id}/workspace/branches", withRole("observer", workspaceHandler.GetBranches))
mux.HandleFunc("GET /api/projects/{id}/workspace/tree", withRole("observer", workspaceHandler.GetFileTree))
```

- [ ] **Step 3: Verify compilation**

Run: `cd internal/server && go build .`

Expected: No compilation errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/routes.go internal/shared/profile/bootstrap.go
git commit -m "feat: register workspace API routes"
```

---

## Task 10: Frontend API Types

**Files:**
- Modify: `frontend/src/shared/api.ts`

- [ ] **Step 1: Add workspace types and API functions**

Add to `frontend/src/shared/api.ts`:

```typescript
// Workspace types
export interface ProjectWorkspace {
  workspace_path: string;
  git_remote_url: string;
  git_branch: string;
}

export interface WorkspaceStatus {
  project_id: string;
  path: string;
  branch: string;
  modified_files: string[];
  staged_files: string[];
  untracked_files: string[];
  last_commit: string;
  last_push: string;
  is_clean: boolean;
  remote_url: string;
  has_remote: boolean;
}

export interface GitCommit {
  hash: string;
  message: string;
  author: string;
  email: string;
  timestamp: string;
}

export interface GitBranch {
  name: string;
  is_current: boolean;
  is_remote: boolean;
  last_commit: string;
}

export interface FileTreeNode {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modified?: string;
  git_status?: 'added' | 'modified' | 'deleted' | 'untracked';
  children?: FileTreeNode[];
  total_files?: number;
  total_dirs?: number;
}

export interface FileTreeResponse {
  root: string;
  path: string;
  tree: FileTreeNode;
}

// Workspace API functions
export const workspaceApi = {
  init: async (projectId: string, gitRemoteUrl?: string): Promise<void> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/init`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ git_remote_url: gitRemoteUrl || '' }),
    });
    if (!response.ok) throw new Error('Failed to initialize workspace');
  },

  getStatus: async (projectId: string): Promise<WorkspaceStatus> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/status`);
    if (!response.ok) throw new Error('Failed to get workspace status');
    return response.json();
  },

  stage: async (projectId: string, files: string[]): Promise<void> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/stage`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ files }),
    });
    if (!response.ok) throw new Error('Failed to stage files');
  },

  commit: async (projectId: string, message: string): Promise<void> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/commit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message }),
    });
    if (!response.ok) throw new Error('Failed to commit');
  },

  push: async (projectId: string): Promise<void> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/push`, {
      method: 'POST',
    });
    if (!response.ok) throw new Error('Failed to push');
  },

  pull: async (projectId: string): Promise<void> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/pull`, {
      method: 'POST',
    });
    if (!response.ok) throw new Error('Failed to pull');
  },

  getDiff: async (projectId: string, file: string): Promise<string> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/diff?file=${encodeURIComponent(file)}`);
    if (!response.ok) throw new Error('Failed to get diff');
    const data = await response.json();
    return data.diff;
  },

  getLog: async (projectId: string, limit?: number): Promise<GitCommit[]> => {
    const url = `/api/projects/${projectId}/workspace/log${limit ? `?limit=${limit}` : ''}`;
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to get log');
    return response.json();
  },

  getBranches: async (projectId: string): Promise<GitBranch[]> => {
    const response = await fetch(`/api/projects/${projectId}/workspace/branches`);
    if (!response.ok) throw new Error('Failed to get branches');
    return response.json();
  },

  getFileTree: async (projectId: string, path?: string, depth?: number): Promise<FileTreeResponse> => {
    let url = `/api/projects/${projectId}/workspace/tree`;
    const params = new URLSearchParams();
    if (path) params.set('path', path);
    if (depth) params.set('depth', depth.toString());
    if (params.toString()) url += `?${params.toString()}`;
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to get file tree');
    return response.json();
  },
};
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/shared/api.ts
git commit -m "feat: add workspace API types and functions"
```

---

## Task 11: Frontend Workspace Context

**Files:**
- Create: `frontend/src/features/workspace/workspace-context.tsx`

- [ ] **Step 1: Create workspace context**

```typescript
// frontend/src/features/workspace/workspace-context.tsx
import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { workspaceApi, WorkspaceStatus, GitCommit, GitBranch } from '../../shared/api';

interface WorkspaceContextType {
  status: WorkspaceStatus | null;
  loading: boolean;
  error: string | null;
  
  // Operations
  refresh: () => Promise<void>;
  stage: (files: string[]) => Promise<void>;
  unstage: (files: string[]) => Promise<void>;
  commit: (message: string) => Promise<void>;
  push: () => Promise<void>;
  pull: () => Promise<void>;
  getDiff: (file: string) => Promise<string>;
  getLog: (limit?: number) => Promise<GitCommit[]>;
  getBranches: () => Promise<GitBranch[]>;
}

const WorkspaceContext = createContext<WorkspaceContextType | null>(null);

export function useWorkspace() {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error('useWorkspace must be used within a WorkspaceProvider');
  }
  return context;
}

interface WorkspaceProviderProps {
  projectId: string;
  children: React.ReactNode;
}

export function WorkspaceProvider({ projectId, children }: WorkspaceProviderProps) {
  const [status, setStatus] = useState<WorkspaceStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const newStatus = await workspaceApi.getStatus(projectId);
      setStatus(newStatus);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get status');
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const stage = useCallback(async (files: string[]) => {
    try {
      await workspaceApi.stage(projectId, files);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to stage files');
      throw err;
    }
  }, [projectId, refresh]);

  const unstage = useCallback(async (files: string[]) => {
    // Git doesn't have a direct unstage API via our interface
    // This would need to be implemented via git reset
    console.warn('Unstage not implemented yet');
  }, []);

  const commit = useCallback(async (message: string) => {
    try {
      await workspaceApi.commit(projectId, message);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit');
      throw err;
    }
  }, [projectId, refresh]);

  const push = useCallback(async () => {
    try {
      await workspaceApi.push(projectId);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to push');
      throw err;
    }
  }, [projectId, refresh]);

  const pull = useCallback(async () => {
    try {
      await workspaceApi.pull(projectId);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to pull');
      throw err;
    }
  }, [projectId, refresh]);

  const getDiff = useCallback(async (file: string) => {
    return workspaceApi.getDiff(projectId, file);
  }, [projectId]);

  const getLog = useCallback(async (limit?: number) => {
    return workspaceApi.getLog(projectId, limit);
  }, [projectId]);

  const getBranches = useCallback(async () => {
    return workspaceApi.getBranches(projectId);
  }, [projectId]);

  const value: WorkspaceContextType = {
    status,
    loading,
    error,
    refresh,
    stage,
    unstage,
    commit,
    push,
    pull,
    getDiff,
    getLog,
    getBranches,
  };

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/workspace-context.tsx
git commit -m "feat: add workspace context for state management"
```

---

## Task 12: GitPanel Component

**Files:**
- Create: `frontend/src/features/workspace/GitPanel.tsx`
- Create: `frontend/src/features/workspace/GitToolbar.tsx`
- Create: `frontend/src/features/workspace/GitCommitInput.tsx`

- [ ] **Step 1: Create GitToolbar component**

```typescript
// frontend/src/features/workspace/GitToolbar.tsx
import React from 'react';

interface GitToolbarProps {
  onPush: () => void;
  onPull: () => void;
  onRefresh: () => void;
  isClean: boolean;
  loading: boolean;
}

export function GitToolbar({ onPush, onPull, onRefresh, isClean, loading }: GitToolbarProps) {
  return (
    <div className="git-toolbar">
      <button 
        onClick={onRefresh} 
        disabled={loading}
        title="Refresh status"
      >
        ↻
      </button>
      <button 
        onClick={onPull} 
        disabled={loading}
        title="Pull from remote"
      >
        ↓ Pull
      </button>
      <button 
        onClick={onPush} 
        disabled={loading || isClean}
        title="Push to remote"
      >
        ↑ Push
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Create GitCommitInput component**

```typescript
// frontend/src/features/workspace/GitCommitInput.tsx
import React, { useState } from 'react';

interface GitCommitInputProps {
  onCommit: (message: string) => Promise<void>;
  disabled: boolean;
}

export function GitCommitInput({ onCommit, disabled }: GitCommitInputProps) {
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!message.trim()) return;
    
    setLoading(true);
    try {
      await onCommit(message);
      setMessage('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="git-commit-input">
      <input
        type="text"
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder="Commit message..."
        disabled={disabled || loading}
      />
      <button type="submit" disabled={disabled || loading || !message.trim()}>
        {loading ? 'Committing...' : 'Commit'}
      </button>
    </form>
  );
}
```

- [ ] **Step 3: Create GitPanel component**

```typescript
// frontend/src/features/workspace/GitPanel.tsx
import React, { useState } from 'react';
import { useWorkspace } from './workspace-context';
import { GitToolbar } from './GitToolbar';
import { GitCommitInput } from './GitCommitInput';
import { GitStatus } from './GitStatus';

interface GitPanelProps {
  projectId: string;
}

export function GitPanel({ projectId }: GitPanelProps) {
  const { status, loading, error, refresh, stage, commit, push, pull } = useWorkspace();
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());

  const toggleFile = (file: string) => {
    setSelectedFiles(prev => {
      const next = new Set(prev);
      if (next.has(file)) {
        next.delete(file);
      } else {
        next.add(file);
      }
      return next;
    });
  };

  const stageSelected = async () => {
    await stage(Array.from(selectedFiles));
    setSelectedFiles(new Set());
  };

  const handlePush = async () => {
    try {
      await push();
    } catch (err) {
      // Error is handled by context
    }
  };

  const handlePull = async () => {
    try {
      await pull();
    } catch (err) {
      // Error is handled by context
    }
  };

  if (error) {
    return (
      <div className="git-panel error">
        <p>Error: {error}</p>
        <button onClick={refresh}>Retry</button>
      </div>
    );
  }

  return (
    <div className="git-panel">
      <GitToolbar 
        onPush={handlePush}
        onPull={handlePull}
        onRefresh={refresh}
        isClean={status?.is_clean ?? true}
        loading={loading}
      />
      
      <GitStatus 
        status={status}
        selectedFiles={selectedFiles}
        onToggleFile={toggleFile}
        onStageSelected={stageSelected}
      />
      
      <GitCommitInput 
        onCommit={commit}
        disabled={loading || (status?.is_clean ?? true)}
      />
    </div>
  );
}
```

- [ ] **Step 4: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/workspace/GitPanel.tsx frontend/src/features/workspace/GitToolbar.tsx frontend/src/features/workspace/GitCommitInput.tsx
git commit -m "feat: add GitPanel, GitToolbar, and GitCommitInput components"
```

---

## Task 13: GitStatus Component

**Files:**
- Create: `frontend/src/features/workspace/GitStatus.tsx`

- [ ] **Step 1: Create GitStatus component**

```typescript
// frontend/src/features/workspace/GitStatus.tsx
import React from 'react';
import { WorkspaceStatus } from '../../shared/api';

interface GitStatusProps {
  status: WorkspaceStatus | null;
  selectedFiles: Set<string>;
  onToggleFile: (file: string) => void;
  onStageSelected: () => void;
}

export function GitStatus({ status, selectedFiles, onToggleFile, onStageSelected }: GitStatusProps) {
  if (!status) {
    return <div className="git-status">Loading...</div>;
  }

  const hasChanges = !status.is_clean;
  const hasSelected = selectedFiles.size > 0;

  return (
    <div className="git-status">
      <div className="git-status-header">
        <span className="branch">{status.branch}</span>
        {status.has_remote && <span className="remote">origin</span>}
        {hasChanges && <span className="changes">{status.modified_files.length + status.staged_files.length + status.untracked_files.length} changes</span>}
      </div>

      {status.staged_files.length > 0 && (
        <div className="file-group">
          <h4>Staged Changes</h4>
          {status.staged_files.map(file => (
            <div 
              key={file} 
              className={`file-item staged ${selectedFiles.has(file) ? 'selected' : ''}`}
              onClick={() => onToggleFile(file)}
            >
              <span className="file-icon">✓</span>
              <span className="file-name">{file}</span>
            </div>
          ))}
        </div>
      )}

      {status.modified_files.length > 0 && (
        <div className="file-group">
          <h4>Changes</h4>
          {status.modified_files.map(file => (
            <div 
              key={file} 
              className={`file-item modified ${selectedFiles.has(file) ? 'selected' : ''}`}
              onClick={() => onToggleFile(file)}
            >
              <span className="file-icon">M</span>
              <span className="file-name">{file}</span>
            </div>
          ))}
        </div>
      )}

      {status.untracked_files.length > 0 && (
        <div className="file-group">
          <h4>Untracked Files</h4>
          {status.untracked_files.map(file => (
            <div 
              key={file} 
              className={`file-item untracked ${selectedFiles.has(file) ? 'selected' : ''}`}
              onClick={() => onToggleFile(file)}
            >
              <span className="file-icon">?</span>
              <span className="file-name">{file}</span>
            </div>
          ))}
        </div>
      )}

      {hasSelected && (
        <div className="git-status-actions">
          <button onClick={onStageSelected}>
            Stage {selectedFiles.size} file{selectedFiles.size > 1 ? 's' : ''}
          </button>
        </div>
      )}

      {status.is_clean && (
        <div className="git-status-clean">
          <span>✓ Working tree clean</span>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/GitStatus.tsx
git commit -m "feat: add GitStatus component"
```

---

## Task 14: GitDiffView Component

**Files:**
- Create: `frontend/src/features/workspace/GitDiffView.tsx`

- [ ] **Step 1: Create GitDiffView component**

```typescript
// frontend/src/features/workspace/GitDiffView.tsx
import React, { useState, useEffect } from 'react';
import { workspaceApi } from '../../shared/api';

interface GitDiffViewProps {
  projectId: string;
  file: string;
  onClose: () => void;
}

export function GitDiffView({ projectId, file, onClose }: GitDiffViewProps) {
  const [diff, setDiff] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadDiff();
  }, [projectId, file]);

  const loadDiff = async () => {
    setLoading(true);
    setError(null);
    try {
      const diffContent = await workspaceApi.getDiff(projectId, file);
      setDiff(diffContent);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load diff');
    } finally {
      setLoading(false);
    }
  };

  const renderDiff = (diffContent: string) => {
    if (!diffContent) return <div className="diff-empty">No changes</div>;
    
    const lines = diffContent.split('\n');
    return (
      <pre className="diff-content">
        {lines.map((line, index) => {
          let className = 'diff-line';
          if (line.startsWith('+')) className += ' diff-add';
          else if (line.startsWith('-')) className += ' diff-remove';
          else if (line.startsWith('@@')) className += ' diff-hunk';
          
          return (
            <div key={index} className={className}>
              <span className="diff-line-number">{index + 1}</span>
              <span className="diff-line-content">{line}</span>
            </div>
          );
        })}
      </pre>
    );
  };

  return (
    <div className="git-diff-view">
      <div className="diff-header">
        <h3>Changes: {file}</h3>
        <button onClick={onClose} className="diff-close">×</button>
      </div>
      
      {loading && <div className="diff-loading">Loading diff...</div>}
      {error && <div className="diff-error">Error: {error}</div>}
      {diff && renderDiff(diff)}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/GitDiffView.tsx
git commit -m "feat: add GitDiffView component for viewing file changes"
```

---

## Task 15: GitHistory Component

**Files:**
- Create: `frontend/src/features/workspace/GitHistory.tsx`

- [ ] **Step 1: Create GitHistory component**

```typescript
// frontend/src/features/workspace/GitHistory.tsx
import React, { useState, useEffect } from 'react';
import { workspaceApi, GitCommit } from '../../shared/api';

interface GitHistoryProps {
  projectId: string;
  limit?: number;
}

export function GitHistory({ projectId, limit = 20 }: GitHistoryProps) {
  const [commits, setCommits] = useState<GitCommit[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadHistory();
  }, [projectId, limit]);

  const loadHistory = async () => {
    setLoading(true);
    setError(null);
    try {
      const history = await workspaceApi.getLog(projectId, limit);
      setCommits(history);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load history');
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const formatHash = (hash: string) => {
    return hash.substring(0, 7);
  };

  if (loading) {
    return <div className="git-history loading">Loading history...</div>;
  }

  if (error) {
    return (
      <div className="git-history error">
        <p>Error: {error}</p>
        <button onClick={loadHistory}>Retry</button>
      </div>
    );
  }

  return (
    <div className="git-history">
      <div className="history-header">
        <h3>Commit History</h3>
        <span className="commit-count">{commits.length} commits</span>
      </div>
      
      <div className="commit-list">
        {commits.map((commit) => (
          <div key={commit.hash} className="commit-item">
            <div className="commit-info">
              <div className="commit-message">{commit.message}</div>
              <div className="commit-meta">
                <span className="commit-hash">{formatHash(commit.hash)}</span>
                <span className="commit-author">{commit.author}</span>
                <span className="commit-date">{formatDate(commit.timestamp)}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
      
      {commits.length === 0 && (
        <div className="history-empty">No commits yet</div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/GitHistory.tsx
git commit -m "feat: add GitHistory component for commit timeline"
```

---

## Task 16: BranchManager Component

**Files:**
- Create: `frontend/src/features/workspace/BranchManager.tsx`

- [ ] **Step 1: Create BranchManager component**

```typescript
// frontend/src/features/workspace/BranchManager.tsx
import React, { useState, useEffect } from 'react';
import { workspaceApi, GitBranch } from '../../shared/api';

interface BranchManagerProps {
  projectId: string;
  onBranchChange?: (branch: string) => void;
}

export function BranchManager({ projectId, onBranchChange }: BranchManagerProps) {
  const [branches, setBranches] = useState<GitBranch[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newBranchName, setNewBranchName] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);

  useEffect(() => {
    loadBranches();
  }, [projectId]);

  const loadBranches = async () => {
    setLoading(true);
    setError(null);
    try {
      const branchList = await workspaceApi.getBranches(projectId);
      setBranches(branchList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load branches');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateBranch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBranchName.trim()) return;
    
    try {
      // This would need a backend endpoint for branch creation
      // For now, we'll just show the form
      console.log('Create branch:', newBranchName);
      setNewBranchName('');
      setShowCreateForm(false);
      await loadBranches();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create branch');
    }
  };

  const handleSwitchBranch = async (branchName: string) => {
    try {
      // This would need a backend endpoint for branch switching
      console.log('Switch to branch:', branchName);
      onBranchChange?.(branchName);
      await loadBranches();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to switch branch');
    }
  };

  const currentBranch = branches.find(b => b.is_current);
  const localBranches = branches.filter(b => !b.is_remote);
  const remoteBranches = branches.filter(b => b.is_remote);

  if (loading) {
    return <div className="branch-manager loading">Loading branches...</div>;
  }

  return (
    <div className="branch-manager">
      <div className="branch-header">
        <h3>Branches</h3>
        <button 
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="branch-create-btn"
        >
          + New Branch
        </button>
      </div>

      {error && <div className="branch-error">Error: {error}</div>}

      {showCreateForm && (
        <form onSubmit={handleCreateBranch} className="branch-create-form">
          <input
            type="text"
            value={newBranchName}
            onChange={(e) => setNewBranchName(e.target.value)}
            placeholder="New branch name"
            required
          />
          <button type="submit">Create</button>
          <button type="button" onClick={() => setShowCreateForm(false)}>Cancel</button>
        </form>
      )}

      {currentBranch && (
        <div className="current-branch">
          <span className="branch-label">Current:</span>
          <span className="branch-name current">{currentBranch.name}</span>
        </div>
      )}

      <div className="branch-section">
        <h4>Local Branches</h4>
        {localBranches.map(branch => (
          <div 
            key={branch.name} 
            className={`branch-item ${branch.is_current ? 'current' : ''}`}
            onClick={() => !branch.is_current && handleSwitchBranch(branch.name)}
          >
            <span className="branch-icon">{branch.is_current ? '✓' : '○'}</span>
            <span className="branch-name">{branch.name}</span>
            <span className="branch-commit">{branch.last_commit?.substring(0, 7)}</span>
          </div>
        ))}
        {localBranches.length === 0 && (
          <div className="branch-empty">No local branches</div>
        )}
      </div>

      <div className="branch-section">
        <h4>Remote Branches</h4>
        {remoteBranches.map(branch => (
          <div key={branch.name} className="branch-item remote">
            <span className="branch-icon">↗</span>
            <span className="branch-name">{branch.name}</span>
            <span className="branch-commit">{branch.last_commit?.substring(0, 7)}</span>
          </div>
        ))}
        {remoteBranches.length === 0 && (
          <div className="branch-empty">No remote branches</div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/BranchManager.tsx
git commit -m "feat: add BranchManager component for branch operations"
```

---

## Task 17: Enhanced FileTreePanel (Phase 2.5)

**Files:**
- Create: `frontend/src/features/workspace/FileTreePanel.tsx`

- [ ] **Step 1: Create enhanced FileTreePanel component**

```typescript
// frontend/src/features/workspace/FileTreePanel.tsx
import React, { useState, useEffect } from 'react';
import { workspaceApi, FileTreeNode, FileTreeResponse } from '../../shared/api';

interface FileTreePanelProps {
  projectId: string;
  onFileSelect?: (file: FileTreeNode) => void;
  depth?: number;
}

export function FileTreePanel({ projectId, onFileSelect, depth = 3 }: FileTreePanelProps) {
  const [treeData, setTreeData] = useState<FileTreeResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());

  useEffect(() => {
    loadTree();
  }, [projectId, depth]);

  const loadTree = async (path?: string) => {
    setLoading(true);
    setError(null);
    try {
      const response = await workspaceApi.getFileTree(projectId, path, depth);
      setTreeData(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load file tree');
    } finally {
      setLoading(false);
    }
  };

  const toggleExpand = (path: string) => {
    setExpandedPaths(prev => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const handleFileClick = (node: FileTreeNode) => {
    if (node.type === 'directory') {
      toggleExpand(node.path);
    } else {
      onFileSelect?.(node);
    }
  };

  const getGitStatusIcon = (status?: string) => {
    switch (status) {
      case 'added': return '✓';
      case 'modified': return 'M';
      case 'deleted': return 'D';
      case 'untracked': return '?';
      default: return '';
    }
  };

  const getGitStatusClass = (status?: string) => {
    switch (status) {
      case 'added': return 'git-added';
      case 'modified': return 'git-modified';
      case 'deleted': return 'git-deleted';
      case 'untracked': return 'git-untracked';
      default: return '';
    }
  };

  const renderNode = (node: FileTreeNode, level: number = 0) => {
    const isExpanded = expandedPaths.has(node.path);
    const hasChildren = node.children && node.children.length > 0;
    const gitStatusIcon = getGitStatusIcon(node.git_status);
    const gitStatusClass = getGitStatusClass(node.git_status);

    return (
      <div key={node.path} className="file-tree-node">
        <div 
          className={`node-content ${gitStatusClass}`}
          style={{ paddingLeft: `${level * 16}px` }}
          onClick={() => handleFileClick(node)}
        >
          <span className="node-icon">
            {node.type === 'directory' ? (isExpanded ? '📂' : '📁') : '📄'}
          </span>
          <span className="node-name">{node.name}</span>
          {gitStatusIcon && (
            <span className={`git-status ${gitStatusClass}`}>{gitStatusIcon}</span>
          )}
          {node.type === 'directory' && node.total_files !== undefined && (
            <span className="node-count">{node.total_files} files</span>
          )}
        </div>
        
        {isExpanded && hasChildren && (
          <div className="node-children">
            {node.children!.map(child => renderNode(child, level + 1))}
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return <div className="file-tree-panel loading">Loading file tree...</div>;
  }

  if (error) {
    return (
      <div className="file-tree-panel error">
        <p>Error: {error}</p>
        <button onClick={() => loadTree()}>Retry</button>
      </div>
    );
  }

  if (!treeData) {
    return <div className="file-tree-panel empty">No workspace found</div>;
  }

  return (
    <div className="file-tree-panel">
      <div className="tree-header">
        <h3>File Explorer</h3>
        <button onClick={() => loadTree()} className="refresh-btn">
          ↻ Refresh
        </button>
      </div>
      
      <div className="tree-root">
        <div className="root-path">{treeData.root}</div>
        {treeData.tree && renderNode(treeData.tree)}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/FileTreePanel.tsx
git commit -m "feat: add enhanced FileTreePanel with Git status integration"
```

---

## Task 18: Unit Tests

**Files:**
- Create: `internal/workspace/service/workspace_manager_test.go`
- Create: `internal/workspace/adapter/git_executor_test.go`
- Create: `internal/workspace/service/file_tree_service_test.go`

- [ ] **Step 1: Create WorkspaceManager unit tests**

```go
// internal/workspace/service/workspace_manager_test.go
package service

import (
	"context"
	"testing"
	
	"openforge/internal/workspace/domain"
)

// MockWorkspaceRepository implements port.WorkspaceRepository for testing
type MockWorkspaceRepository struct {
	config    *domain.WorkspaceConfig
	workspaces map[string]*domain.Workspace
}

func NewMockWorkspaceRepository() *MockWorkspaceRepository {
	return &MockWorkspaceRepository{
		config: &domain.WorkspaceConfig{
			ID:                 "test-config-id",
			RootPath:           "/tmp/test-workspace",
			AutoPushInterval:   30,
			AutoStage:          true,
			AutoPush:           false,
			MaxWorkspaceSizeMB: 5000,
		},
		workspaces: make(map[string]*domain.Workspace),
	}
}

func (m *MockWorkspaceRepository) GetConfig(ctx context.Context) (*domain.WorkspaceConfig, error) {
	return m.config, nil
}

func (m *MockWorkspaceRepository) UpdateConfig(ctx context.Context, config *domain.WorkspaceConfig) error {
	m.config = config
	return nil
}

func (m *MockWorkspaceRepository) GetWorkspace(ctx context.Context, projectID string) (*domain.Workspace, error) {
	ws, ok := m.workspaces[projectID]
	if !ok {
		return nil, domain.ErrWorkspaceNotFound
	}
	return ws, nil
}

func (m *MockWorkspaceRepository) CreateWorkspace(ctx context.Context, ws *domain.Workspace) error {
	m.workspaces[ws.ProjectID] = ws
	return nil
}

func (m *MockWorkspaceRepository) UpdateWorkspace(ctx context.Context, ws *domain.Workspace) error {
	m.workspaces[ws.ProjectID] = ws
	return nil
}

func (m *MockWorkspaceRepository) DeleteWorkspace(ctx context.Context, projectID string) error {
	delete(m.workspaces, projectID)
	return nil
}

func (m *MockWorkspaceRepository) UpdateProjectWorkspace(ctx context.Context, projectID, workspacePath, gitRemote, gitBranch string) error {
	if ws, ok := m.workspaces[projectID]; ok {
		ws.Path = workspacePath
		ws.GitRemote = gitRemote
		ws.GitBranch = gitBranch
	}
	return nil
}

func (m *MockWorkspaceRepository) GetProjectWorkspace(ctx context.Context, projectID string) (string, string, string, error) {
	ws, ok := m.workspaces[projectID]
	if !ok {
		return "", "", "", domain.ErrWorkspaceNotFound
	}
	return ws.Path, ws.GitRemote, ws.GitBranch, nil
}

func TestWorkspaceManager_InitWorkspace(t *testing.T) {
	repo := NewMockWorkspaceRepository()
	wm, err := NewWorkspaceManager(repo)
	if err != nil {
		t.Fatalf("Failed to create WorkspaceManager: %v", err)
	}
	
	ctx := context.Background()
	projectID := "test-project-1"
	
	// Test initialization
	err = wm.InitWorkspace(ctx, projectID, "https://github.com/test/repo.git")
	if err != nil {
		t.Fatalf("Failed to init workspace: %v", err)
	}
	
	// Test duplicate initialization
	err = wm.InitWorkspace(ctx, projectID, "https://github.com/test/repo.git")
	if err != domain.ErrWorkspaceExists {
		t.Errorf("Expected ErrWorkspaceExists, got: %v", err)
	}
}

func TestWorkspaceManager_GetStatus(t *testing.T) {
	repo := NewMockWorkspaceRepository()
	wm, err := NewWorkspaceManager(repo)
	if err != nil {
		t.Fatalf("Failed to create WorkspaceManager: %v", err)
	}
	
	ctx := context.Background()
	projectID := "test-project-2"
	
	// Initialize workspace first
	err = wm.InitWorkspace(ctx, projectID, "")
	if err != nil {
		t.Fatalf("Failed to init workspace: %v", err)
	}
	
	// Get status
	status, err := wm.GetStatus(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	
	if status.ProjectID != projectID {
		t.Errorf("Expected project ID %s, got %s", projectID, status.ProjectID)
	}
}

func TestWorkspaceManager_NotFound(t *testing.T) {
	repo := NewMockWorkspaceRepository()
	wm, err := NewWorkspaceManager(repo)
	if err != nil {
		t.Fatalf("Failed to create WorkspaceManager: %v", err)
	}
	
	ctx := context.Background()
	
	// Test getting status of non-existent workspace
	_, err = wm.GetStatus(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent workspace")
	}
}
```

- [ ] **Step 2: Create GitExecutor unit tests**

```go
// internal/workspace/adapter/git_executor_test.go
package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitExecutor_Init(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	ge := NewGitExecutor(tmpDir)
	
	// Test init
	err = ge.Init(false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	
	// Verify .git directory exists
	gitDir := filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Git directory not created")
	}
}

func TestGitExecutor_Status(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	ge := NewGitExecutor(tmpDir)
	
	// Init repo
	err = ge.Init(false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	
	// Get status
	status, err := ge.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	
	if !status.IsClean {
		t.Error("New repo should be clean")
	}
}

func TestGitExecutor_AddCommit(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	ge := NewGitExecutor(tmpDir)
	
	// Init repo
	err = ge.Init(false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	
	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add file
	err = ge.Add("test.txt")
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	
	// Commit
	err = ge.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	
	// Verify status is clean
	status, err := ge.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	
	if !status.IsClean {
		t.Error("Repo should be clean after commit")
	}
}
```

- [ ] **Step 3: Create FileTreeService unit tests**

```go
// internal/workspace/service/file_tree_service_test.go
package service

import (
	"os"
	"path/filepath"
	"testing"
	
	"openforge/internal/workspace/domain"
)

func TestFileTreeService_BuildFileTree(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "filetree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Create test structure
	dirs := []string{"src", "src/components", "docs"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}
	
	files := map[string]string{
		"README.md":           "# Test",
		"src/main.go":         "package main",
		"src/components/App.tsx": "export default function App() {}",
		"docs/README.md":      "# Docs",
	}
	
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}
	
	fts := NewFileTreeService()
	
	// Test building tree
	tree, err := fts.BuildFileTree(tmpDir, "", 0, 3)
	if err != nil {
		t.Fatalf("Failed to build file tree: %v", err)
	}
	
	if tree.Name != filepath.Base(tmpDir) {
		t.Errorf("Expected root name %s, got %s", filepath.Base(tmpDir), tree.Name)
	}
	
	if tree.Type != "directory" {
		t.Errorf("Expected root type 'directory', got '%s'", tree.Type)
	}
	
	if tree.TotalFiles != 4 {
		t.Errorf("Expected 4 files, got %d", tree.TotalFiles)
	}
}

func TestFileTreeService_ShouldIgnore(t *testing.T) {
	fts := NewFileTreeService()
	
	ignored := []string{".git", "node_modules", ".DS_Store", "__pycache__", ".hidden"}
	for _, name := range ignored {
		if !fts.shouldIgnore(name) {
			t.Errorf("Expected %s to be ignored", name)
		}
	}
	
	notIgnored := []string{"src", "README.md", "package.json"}
	for _, name := range notIgnored {
		if fts.shouldIgnore(name) {
			t.Errorf("Expected %s to not be ignored", name)
		}
	}
}

func TestFileTreeService_ApplyGitStatus(t *testing.T) {
	fts := NewFileTreeService()
	
	// Create a simple tree
	tree := &domain.FileTreeNode{
		Name: "root",
		Path: "",
		Type: "directory",
		Children: []*domain.FileTreeNode{
			{Name: "file1.txt", Path: "file1.txt", Type: "file"},
			{Name: "file2.txt", Path: "file2.txt", Type: "file"},
			{Name: "file3.txt", Path: "file3.txt", Type: "file"},
		},
	}
	
	// Create git status
	gitStatus := &domain.WorkspaceStatus{
		ModifiedFiles:  []string{"file1.txt"},
		StagedFiles:    []string{"file2.txt"},
		UntrackedFiles: []string{"file3.txt"},
	}
	
	// Apply status
	fts.ApplyGitStatus(tree, gitStatus)
	
	// Verify status was applied
	if tree.Children[0].GitStatus != "modified" {
		t.Errorf("Expected file1.txt to be modified, got %s", tree.Children[0].GitStatus)
	}
	
	if tree.Children[1].GitStatus != "added" {
		t.Errorf("Expected file2.txt to be added, got %s", tree.Children[1].GitStatus)
	}
	
	if tree.Children[2].GitStatus != "untracked" {
		t.Errorf("Expected file3.txt to be untracked, got %s", tree.Children[2].GitStatus)
	}
}
```

- [ ] **Step 4: Run unit tests**

Run: `cd internal/workspace && go test ./...`

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace/service/workspace_manager_test.go internal/workspace/adapter/git_executor_test.go internal/workspace/service/file_tree_service_test.go
git commit -m "test: add unit tests for workspace services"
```

---

## Task 19: Integration Tests

**Files:**
- Create: `frontend/src/features/workspace/__tests__/GitPanel.test.tsx`

- [ ] **Step 1: Create GitPanel integration test**

```typescript
// frontend/src/features/workspace/__tests__/GitPanel.test.tsx
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { GitPanel } from '../GitPanel';
import { WorkspaceProvider } from '../workspace-context';

// Mock the workspace API
jest.mock('../../../shared/api', () => ({
  workspaceApi: {
    getStatus: jest.fn(),
    stage: jest.fn(),
    commit: jest.fn(),
    push: jest.fn(),
    pull: jest.fn(),
  },
}));

const mockStatus = {
  project_id: 'test-project',
  path: '/tmp/test',
  branch: 'main',
  modified_files: ['file1.txt'],
  staged_files: ['file2.txt'],
  untracked_files: ['file3.txt'],
  last_commit: 'abc123',
  last_push: '',
  is_clean: false,
  remote_url: 'https://github.com/test/repo.git',
  has_remote: true,
};

describe('GitPanel', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    
    // Setup default mock implementation
    const { workspaceApi } = require('../../../shared/api');
    workspaceApi.getStatus.mockResolvedValue(mockStatus);
    workspaceApi.stage.mockResolvedValue(undefined);
    workspaceApi.commit.mockResolvedValue(undefined);
    workspaceApi.push.mockResolvedValue(undefined);
    workspaceApi.pull.mockResolvedValue(undefined);
  });

  test('renders loading state initially', () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  test('renders git status after loading', async () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByText('main')).toBeInTheDocument();
      expect(screen.getByText('3 changes')).toBeInTheDocument();
    });
  });

  test('can stage files', async () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByText('file1.txt')).toBeInTheDocument();
    });
    
    // Click on a file to select it
    fireEvent.click(screen.getByText('file1.txt'));
    
    // Click stage button
    const stageButton = screen.getByText(/Stage \d+ file/);
    fireEvent.click(stageButton);
    
    await waitFor(() => {
      const { workspaceApi } = require('../../../shared/api');
      expect(workspaceApi.stage).toHaveBeenCalledWith('test-project', ['file1.txt']);
    });
  });

  test('can commit changes', async () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Commit message...')).toBeInTheDocument();
    });
    
    // Type commit message
    const input = screen.getByPlaceholderText('Commit message...');
    fireEvent.change(input, { target: { value: 'Test commit' } });
    
    // Click commit button
    const commitButton = screen.getByText('Commit');
    fireEvent.click(commitButton);
    
    await waitFor(() => {
      const { workspaceApi } = require('../../../shared/api');
      expect(workspaceApi.commit).toHaveBeenCalledWith('test-project', 'Test commit');
    });
  });

  test('can push changes', async () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByText('↑ Push')).toBeInTheDocument();
    });
    
    // Click push button
    fireEvent.click(screen.getByText('↑ Push'));
    
    await waitFor(() => {
      const { workspaceApi } = require('../../../shared/api');
      expect(workspaceApi.push).toHaveBeenCalledWith('test-project');
    });
  });

  test('can pull changes', async () => {
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByText('↓ Pull')).toBeInTheDocument();
    });
    
    // Click pull button
    fireEvent.click(screen.getByText('↓ Pull'));
    
    await waitFor(() => {
      const { workspaceApi } = require('../../../shared/api');
      expect(workspaceApi.pull).toHaveBeenCalledWith('test-project');
    });
  });

  test('displays error state', async () => {
    const { workspaceApi } = require('../../../shared/api');
    workspaceApi.getStatus.mockRejectedValue(new Error('Network error'));
    
    render(
      <WorkspaceProvider projectId="test-project">
        <GitPanel projectId="test-project" />
      </WorkspaceProvider>
    );
    
    await waitFor(() => {
      expect(screen.getByText(/Error: Network error/)).toBeInTheDocument();
      expect(screen.getByText('Retry')).toBeInTheDocument();
    });
  });
});
```

- [ ] **Step 2: Run frontend tests**

Run: `cd frontend && npm test`

Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/workspace/__tests__/GitPanel.test.tsx
git commit -m "test: add GitPanel integration tests"
```

---

## Task 20: End-to-end verification

**Files:**
- Modify: `internal/shared/profile/bootstrap.go` (if needed)
- Modify: `internal/server/routes.go` (if needed)

- [ ] **Step 1: Verify backend compilation**

Run: `cd internal && go build ./...`

Expected: No compilation errors.

- [ ] **Step 2: Verify frontend compilation**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Run all backend tests**

Run: `cd internal && go test ./...`

Expected: All tests pass.

- [ ] **Step 4: Run all frontend tests**

Run: `cd frontend && npm test`

Expected: All tests pass.

- [ ] **Step 5: Manual verification**

1. Start the server: `go run cmd/server/main.go`
2. Open frontend: `cd frontend && npm run dev`
3. Test workspace initialization:
   - Create a new project
   - Initialize workspace with Git remote
   - Verify workspace path is created
4. Test Git operations:
   - Create some files in the workspace
   - Check Git status shows untracked files
   - Stage files
   - Commit changes
   - Push to remote (if configured)
5. Test file tree:
   - Browse workspace files
   - Verify Git status indicators
   - Click on files to view content
6. Test branch management:
   - View branches
   - Create new branch (if implemented)
   - Switch branches (if implemented)

- [ ] **Step 6: Create final commit**

```bash
git add .
git commit -m "feat: complete workspace management implementation"
```

- [ ] **Step 7: Create pull request**

If on a feature branch, create a pull request to merge into main.

---

## Summary

This implementation plan provides a complete workspace management system with Git integration. The system includes:

1. **Database schema** for workspace configuration and project workspace fields
2. **Backend services** for workspace management, Git operations, and file tree building
3. **Frontend components** for Git status, commit history, branch management, and file browsing
4. **API endpoints** for all workspace operations
5. **Comprehensive testing** at unit, integration, and end-to-end levels

The implementation follows the **Database-driven + Filesystem abstraction** architecture, providing each project with its own isolated workspace while maintaining a global configuration layer.

**Total tasks:** 20
**Estimated effort:** 2-3 days for a single developer
**Dependencies:** PostgreSQL, Git CLI, Node.js, Go

After completing this implementation, users will be able to:
- Initialize workspaces for projects with Git integration
- View real-time Git status and file changes
- Stage, commit, and push changes
- Browse workspace files with Git status indicators
- Manage branches (future enhancement)
- View commit history and diffs