package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"openforge/internal/shared/kernel"
	observabilitydomain "openforge/internal/observability/domain"
)

type mockCommandExecutor struct {
	failCount int
	calls     int
}

func (m *mockCommandExecutor) Execute(ctx context.Context, command string, opts kernel.ExecOptions) (kernel.ExecOutput, error) {
	m.calls++
	if m.failCount > 0 {
		m.failCount--
		return kernel.ExecOutput{}, errors.New("exec error")
	}
	return kernel.ExecOutput{ExitCode: 0, Stdout: "success"}, nil
}

func (m *mockCommandExecutor) ExecuteStream(ctx context.Context, command string, opts kernel.ExecOptions) (<-chan kernel.ExecStreamChunk, error) {
	return nil, nil
}

func (m *mockCommandExecutor) Validate(ctx context.Context, command string, opts kernel.ExecOptions) error {
	return nil
}

func TestBreakerCommandExecutor_DelegatesWhenClosed(t *testing.T) {
	breaker := observabilitydomain.NewBreaker(observabilitydomain.BreakerConfig{
		MaxFailures: 3,
	})
	mock := &mockCommandExecutor{}
	wrapper := &breakerCommandExecutor{name: "test", breaker: breaker, next: mock}

	out, err := wrapper.Execute(context.Background(), "ls", kernel.ExecOptions{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Stdout != "success" {
		t.Fatalf("expected 'success', got %s", out.Stdout)
	}
	if mock.calls != 1 {
		t.Fatalf("expected 1 call to mock, got %d", mock.calls)
	}
}

func TestBreakerCommandExecutor_RejectsWhenOpen(t *testing.T) {
	breaker := observabilitydomain.NewBreaker(observabilitydomain.BreakerConfig{
		MaxFailures: 2,
		OpenDuration: 1 * time.Second,
	})
	mock := &mockCommandExecutor{failCount: 5}
	wrapper := &breakerCommandExecutor{name: "test", breaker: breaker, next: mock}

	// 1st fail
	_, _ = wrapper.Execute(context.Background(), "ls", kernel.ExecOptions{})
	// 2nd fail -> should trip open
	_, _ = wrapper.Execute(context.Background(), "ls", kernel.ExecOptions{})

	if breaker.State() != observabilitydomain.StateOpen {
		t.Fatalf("expected breaker to be open, got %v", breaker.State())
	}

	// 3rd call -> should reject with ErrCircuitOpen instantly without delegating
	_, err := wrapper.Execute(context.Background(), "ls", kernel.ExecOptions{})
	if !errors.Is(err, observabilitydomain.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if mock.calls != 2 {
		t.Fatalf("expected mock to only have 2 calls, got %d", mock.calls)
	}
}
