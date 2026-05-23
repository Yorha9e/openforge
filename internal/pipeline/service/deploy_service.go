package service

import (
	"context"
	"fmt"
	"time"

	"openforge/internal/shared/kernel"
)

// DeployResult holds the outcome of a deployment attempt.
type DeployResult struct {
	Status      string        // "deployed" | "rolled_back"
	DryRunOut   string
	ApplyOut    string
	VerifyOut   string
	RollbackOut string
	Duration    time.Duration
}

// DeployService orchestrates the four-step deploy pipeline.
type DeployService struct {
	exec kernel.CommandExecutor
}

// NewDeployService creates a DeployService backed by the given executor.
func NewDeployService(exec kernel.CommandExecutor) *DeployService {
	return &DeployService{exec: exec}
}

// Deploy runs: pre-apply dry-run -> apply -> post-apply verify -> rollback on failure.
func (s *DeployService) Deploy(ctx context.Context, projectID, worktreePath, branch string) (*DeployResult, error) {
	start := time.Now()
	result := &DeployResult{}

	// Step 1: Dry-run
	dryOut, err := s.exec.Execute(ctx,
		fmt.Sprintf("cd %q && bash _apply.sh --dry-run --branch %s", worktreePath, branch),
		kernel.ExecOptions{WorkDir: worktreePath, Timeout: 60 * time.Second},
	)
	if err != nil || dryOut.ExitCode != 0 {
		return nil, fmt.Errorf("dry-run failed: %s (exit %d)", dryOut.Stderr, dryOut.ExitCode)
	}
	result.DryRunOut = dryOut.Stdout

	// Step 2: Apply
	applyOut, err := s.exec.Execute(ctx,
		fmt.Sprintf("cd %q && bash _apply.sh --branch %s", worktreePath, branch),
		kernel.ExecOptions{WorkDir: worktreePath, Timeout: 5 * time.Minute},
	)
	if err != nil || applyOut.ExitCode != 0 {
		return nil, fmt.Errorf("apply failed: %s", applyOut.Stderr)
	}
	result.ApplyOut = applyOut.Stdout

	// Step 3: Post-apply verify
	verifyOut, err := s.exec.Execute(ctx,
		fmt.Sprintf("cd %q && bash _verify.sh", worktreePath),
		kernel.ExecOptions{WorkDir: worktreePath, Timeout: 120 * time.Second},
	)
	if err == nil && verifyOut.ExitCode == 0 {
		result.Status = "deployed"
		result.VerifyOut = verifyOut.Stdout
		result.Duration = time.Since(start)
		return result, nil
	}
	result.VerifyOut = verifyOut.Stdout + "\n" + verifyOut.Stderr

	// Step 4: Rollback on verify failure
	rollOut, err := s.exec.Execute(ctx,
		fmt.Sprintf("cd %q && bash _rollback.sh", worktreePath),
		kernel.ExecOptions{WorkDir: worktreePath, Timeout: 120 * time.Second},
	)
	if err != nil || rollOut.ExitCode != 0 {
		return nil, fmt.Errorf("rollback failed after verify failure: %s", rollOut.Stderr)
	}
	result.Status = "rolled_back"
	result.RollbackOut = rollOut.Stdout
	result.Duration = time.Since(start)
	return result, nil
}
