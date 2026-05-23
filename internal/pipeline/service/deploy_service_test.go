package service

import (
	"context"
	"errors"
	"testing"

	"openforge/internal/shared/kernel"
)

type stubCommandExecutor struct {
	results []kernel.ExecOutput
	errors  []error
	callIdx int
}

func (s *stubCommandExecutor) Execute(_ context.Context, _ string, _ kernel.ExecOptions) (kernel.ExecOutput, error) {
	if s.callIdx >= len(s.results) {
		return kernel.ExecOutput{}, errors.New("unexpected call")
	}
	r := s.results[s.callIdx]
	var e error
	if s.callIdx < len(s.errors) {
		e = s.errors[s.callIdx]
	}
	s.callIdx++
	return r, e
}

func (s *stubCommandExecutor) ExecuteStream(_ context.Context, _ string, _ kernel.ExecOptions) (<-chan kernel.ExecStreamChunk, error) {
	return nil, nil
}

func (s *stubCommandExecutor) Validate(_ context.Context, _ string, _ kernel.ExecOptions) error {
	return nil
}

func TestDeployService_DryRunFail(t *testing.T) {
	exec := &stubCommandExecutor{
		results: []kernel.ExecOutput{{ExitCode: 1, Stderr: "syntax error"}},
	}
	svc := NewDeployService(exec)

	_, err := svc.Deploy(context.Background(), "proj-1", "/tmp/worktree", "main")
	if err == nil {
		t.Fatal("expected error on dry-run failure")
	}
}

func TestDeployService_ApplySuccess(t *testing.T) {
	exec := &stubCommandExecutor{
		results: []kernel.ExecOutput{
			{ExitCode: 0, Stdout: "dry-run ok"},       // dry-run
			{ExitCode: 0, Stdout: "applied"},           // apply
			{ExitCode: 0, Stdout: "healthy"},           // verify
		},
	}
	svc := NewDeployService(exec)

	result, err := svc.Deploy(context.Background(), "proj-1", "/tmp/worktree", "main")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "deployed" {
		t.Errorf("status = %q, want deployed", result.Status)
	}
}

func TestDeployService_VerifyFails_Rollback(t *testing.T) {
	exec := &stubCommandExecutor{
		results: []kernel.ExecOutput{
			{ExitCode: 0, Stdout: "dry-run ok"},       // dry-run
			{ExitCode: 0, Stdout: "applied"},           // apply
			{ExitCode: 1, Stdout: "unhealthy"},          // verify FAIL
			{ExitCode: 0, Stdout: "rolled back"},        // rollback
		},
	}
	svc := NewDeployService(exec)

	result, err := svc.Deploy(context.Background(), "proj-1", "/tmp/worktree", "main")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "rolled_back" {
		t.Errorf("status = %q, want rolled_back", result.Status)
	}
}
