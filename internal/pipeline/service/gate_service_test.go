package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"openforge/internal/pipeline/domain"
)

// mockGateRepo implements port.GateRepository for testing.
type mockGateRepo struct {
	events          []*domain.GateEvent
	createEventErr  error
	listPendingErr  error
	claimErr        error
	releaseClaimErr error
	lastClaimPipelineID string
	lastClaimStage      string
	lastClaimActor      string
	lastClaimTTL        time.Duration
	lastReleasePipelineID string
	lastReleaseStage     string
	lastReleaseActor     string
	latestHash      string
}

func (m *mockGateRepo) GetLatestHash(_ context.Context, pipelineID string) (string, error) {
	return m.latestHash, nil
}

func (m *mockGateRepo) CreateEvent(_ context.Context, ev *domain.GateEvent) error {
	if m.createEventErr != nil {
		return m.createEventErr
	}
	m.events = append(m.events, ev)
	m.latestHash = ev.ContentHash
	return nil
}

func (m *mockGateRepo) ListByPipeline(_ context.Context, pipelineID string) ([]*domain.GateEvent, error) {
	var result []*domain.GateEvent
	for _, ev := range m.events {
		if ev.PipelineID == pipelineID {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *mockGateRepo) ListPending(_ context.Context, actor string) ([]*domain.GateEvent, error) {
	if m.listPendingErr != nil {
		return nil, m.listPendingErr
	}
	var result []*domain.GateEvent
	for _, ev := range m.events {
		if ev.Decision == "" {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *mockGateRepo) Claim(_ context.Context, pipelineID, stage, actor string, ttl time.Duration) error {
	if m.claimErr != nil {
		return m.claimErr
	}
	m.lastClaimPipelineID = pipelineID
	m.lastClaimStage = stage
	m.lastClaimActor = actor
	m.lastClaimTTL = ttl
	return nil
}

func (m *mockGateRepo) ReleaseClaim(_ context.Context, pipelineID, stage, actor string) error {
	if m.releaseClaimErr != nil {
		return m.releaseClaimErr
	}
	m.lastReleasePipelineID = pipelineID
	m.lastReleaseStage = stage
	m.lastReleaseActor = actor
	return nil
}

func newL3PipelineAtReview(id string) *domain.Pipeline {
	return &domain.Pipeline{
		ID:           id,
		ProjectID:    "proj1",
		Title:        "Test",
		Level:        "L3",
		Status:       "awaiting_review",
		CurrentStage: "impl",
		Version:      1,
		Stages: []domain.Stage{
			{Type: "clarify", Status: "passed"},
			{Type: "decompose", Status: "passed"},
			{Type: "impl", Status: "running"},
			{Type: "test", Status: "pending"},
			{Type: "deploy", Status: "pending"},
			{Type: "verify", Status: "pending"},
		},
	}
}

func TestGateService_Approve(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"p1": newL3PipelineAtReview("p1"),
		},
	}
	svc := NewGateService(gateMock, pipeMock)

	checklist := domain.GateChecklist{
		CodeReviewed:      true,
		SecurityChecked:   true,
		LicenseCleared:    true,
		CodingStandardMet: true,
	}
	err := svc.Approve(context.Background(), "p1", "impl", "alice", checklist, "LGTM")
	if err != nil {
		t.Fatalf("Approve() unexpected error: %v", err)
	}

	// Verify gate event was created
	if len(gateMock.events) != 1 {
		t.Fatalf("expected 1 gate event, got %d", len(gateMock.events))
	}
	ev := gateMock.events[0]
	if ev.Event != "approved" {
		t.Errorf("event type = %q, want %q", ev.Event, "approved")
	}
	if ev.Actor != "alice" {
		t.Errorf("actor = %q, want %q", ev.Actor, "alice")
	}
	if ev.Decision != "approve" {
		t.Errorf("decision = %q, want %q", ev.Decision, "approve")
	}
	if ev.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	if ev.PrevHash == "" {
		t.Error("PrevHash should not be empty")
	}

	// Verify pipeline state was updated
	p := pipeMock.pipelines["p1"]
	if p.Status != "running" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "running")
	}
	if pipeMock.lastUpdateStatus != "running" {
		t.Errorf("UpdateStatus called with %q, want %q", pipeMock.lastUpdateStatus, "running")
	}
}

func TestGateService_Approve_AdvancesStage(t *testing.T) {
	gateMock := &mockGateRepo{}
	p := newL3PipelineAtReview("p1")
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Approve(context.Background(), "p1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err != nil {
		t.Fatalf("Approve() unexpected error: %v", err)
	}

	// After approve, the impl stage should be "passed" and test should be "running"
	if p.Stages[2].Status != "passed" {
		t.Errorf("impl stage status = %q, want %q", p.Stages[2].Status, "passed")
	}
	if p.Stages[3].Status != "running" {
		t.Errorf("test stage status = %q, want %q", p.Stages[3].Status, "running")
	}
	if p.CurrentStage != "test" {
		t.Errorf("current stage = %q, want %q", p.CurrentStage, "test")
	}
}

func TestGateService_Approve_InvalidTransition(t *testing.T) {
	gateMock := &mockGateRepo{}
	p := newL3Pipeline("p1") // Status is "pending", not "awaiting_review"
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Approve(context.Background(), "p1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err == nil {
		t.Fatal("Approve() expected error when pipeline is not awaiting_review")
	}
}

func TestGateService_Reject(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"p1": newL3PipelineAtReview("p1"),
		},
	}
	svc := NewGateService(gateMock, pipeMock)

	comments := []domain.LineComment{
		{FilePath: "main.go", Line: 42, Comment: "security vulnerability", Mark: "critical"},
	}
	err := svc.Reject(context.Background(), "p1", "impl", "bob", comments, "Security issues found")
	if err != nil {
		t.Fatalf("Reject() unexpected error: %v", err)
	}

	// Verify gate event was created
	if len(gateMock.events) != 1 {
		t.Fatalf("expected 1 gate event, got %d", len(gateMock.events))
	}
	ev := gateMock.events[0]
	if ev.Event != "rejected" {
		t.Errorf("event type = %q, want %q", ev.Event, "rejected")
	}
	if ev.Actor != "bob" {
		t.Errorf("actor = %q, want %q", ev.Actor, "bob")
	}
	if ev.Decision != "reject" {
		t.Errorf("decision = %q, want %q", ev.Decision, "reject")
	}
	if len(ev.LineComments) != 1 {
		t.Fatalf("expected 1 line comment, got %d", len(ev.LineComments))
	}
	if ev.LineComments[0].Mark != "critical" {
		t.Errorf("comment mark = %q, want %q", ev.LineComments[0].Mark, "critical")
	}
	if ev.SummaryFeedback != "Security issues found" {
		t.Errorf("summary = %q, want %q", ev.SummaryFeedback, "Security issues found")
	}
	if ev.PrevHash == "" {
		t.Error("PrevHash should not be empty")
	}

	// Verify pipeline state
	p := pipeMock.pipelines["p1"]
	if p.Status != "rejected" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "rejected")
	}
	if pipeMock.lastUpdateStatus != "rejected" {
		t.Errorf("UpdateStatus called with %q, want %q", pipeMock.lastUpdateStatus, "rejected")
	}
}

func TestGateService_Reject_InvalidTransition(t *testing.T) {
	gateMock := &mockGateRepo{}
	p := newL3Pipeline("p1")
	p.Status = "running"
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Reject(context.Background(), "p1", "impl", "bob", nil, "no")
	if err == nil {
		t.Fatal("Reject() expected error when pipeline is not awaiting_review")
	}
}

func TestGateService_Claim(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Claim(context.Background(), "p1", "impl", "alice")
	if err != nil {
		t.Fatalf("Claim() unexpected error: %v", err)
	}

	if gateMock.lastClaimPipelineID != "p1" {
		t.Errorf("Claim called with pipelineID %q, want %q", gateMock.lastClaimPipelineID, "p1")
	}
	if gateMock.lastClaimStage != "impl" {
		t.Errorf("Claim called with stage %q, want %q", gateMock.lastClaimStage, "impl")
	}
	if gateMock.lastClaimActor != "alice" {
		t.Errorf("Claim called with actor %q, want %q", gateMock.lastClaimActor, "alice")
	}
	if gateMock.lastClaimTTL != 30*time.Minute {
		t.Errorf("Claim called with TTL %v, want %v", gateMock.lastClaimTTL, 30*time.Minute)
	}
}

func TestGateService_Claim_RepoError(t *testing.T) {
	gateMock := &mockGateRepo{claimErr: errors.New("already claimed")}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Claim(context.Background(), "p1", "impl", "alice")
	if err == nil {
		t.Fatal("Claim() expected error when gate repo fails")
	}
}

func TestGateService_Release(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Release(context.Background(), "p1", "impl", "alice")
	if err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	if gateMock.lastReleasePipelineID != "p1" {
		t.Errorf("ReleaseClaim called with pipelineID %q, want %q", gateMock.lastReleasePipelineID, "p1")
	}
	if gateMock.lastReleaseStage != "impl" {
		t.Errorf("ReleaseClaim called with stage %q, want %q", gateMock.lastReleaseStage, "impl")
	}
	if gateMock.lastReleaseActor != "alice" {
		t.Errorf("ReleaseClaim called with actor %q, want %q", gateMock.lastReleaseActor, "alice")
	}
}

func TestGateService_Release_RepoError(t *testing.T) {
	gateMock := &mockGateRepo{releaseClaimErr: errors.New("not claimed")}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Release(context.Background(), "p1", "impl", "alice")
	if err == nil {
		t.Fatal("Release() expected error when gate repo fails")
	}
}

func TestGateService_ListPending(t *testing.T) {
	gateMock := &mockGateRepo{
		events: []*domain.GateEvent{
			{PipelineID: "p1", Stage: "impl", Decision: ""},
			{PipelineID: "p2", Stage: "deploy", Decision: ""},
		},
	}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	events, err := svc.ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending() unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 pending events, got %d", len(events))
	}
}

func TestGateService_ListPending_RepoError(t *testing.T) {
	gateMock := &mockGateRepo{listPendingErr: errors.New("db error")}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": newL3Pipeline("p1")},
	}
	svc := NewGateService(gateMock, pipeMock)

	_, err := svc.ListPending(context.Background())
	if err == nil {
		t.Fatal("ListPending() expected error when gate repo fails")
	}
}

func TestGateService_Approve_RepoGetError(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{getByIDErr: errors.New("db unavailable")}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Approve(context.Background(), "p1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err == nil {
		t.Fatal("Approve() expected error when pipeline repo fails")
	}
}

func TestGateService_Reject_RepoGetError(t *testing.T) {
	gateMock := &mockGateRepo{}
	pipeMock := &mockPipelineRepo{getByIDErr: errors.New("db unavailable")}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Reject(context.Background(), "p1", "impl", "bob", nil, "no")
	if err == nil {
		t.Fatal("Reject() expected error when pipeline repo fails")
	}
}

func TestGateService_Approve_CreateEventError(t *testing.T) {
	gateMock := &mockGateRepo{createEventErr: errors.New("event creation failed")}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"p1": newL3PipelineAtReview("p1"),
		},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Approve(context.Background(), "p1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err == nil {
		t.Fatal("Approve() expected error when event creation fails")
	}
}

func TestGateService_Reject_CreateEventError(t *testing.T) {
	gateMock := &mockGateRepo{createEventErr: errors.New("event creation failed")}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"p1": newL3PipelineAtReview("p1"),
		},
	}
	svc := NewGateService(gateMock, pipeMock)

	err := svc.Reject(context.Background(), "p1", "impl", "bob", nil, "no")
	if err == nil {
		t.Fatal("Reject() expected error when event creation fails")
	}
}

func TestGateService_PrevHashChaining(t *testing.T) {
	gateMock := &mockGateRepo{latestHash: "abc123"}
	pipeMock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"pipe-1": newL3PipelineAtReview("pipe-1"),
		},
	}
	svc := NewGateService(gateMock, pipeMock)

	// First approve should use latestHash from repo
	err := svc.Approve(context.Background(), "pipe-1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err != nil {
		t.Fatal(err)
	}
	firstHash := gateMock.events[0].ContentHash
	if gateMock.events[0].PrevHash != "abc123" {
		t.Errorf("first event prevHash = %q, want abc123", gateMock.events[0].PrevHash)
	}

	// Second approve should chain from first
	pipeMock.pipelines["pipe-2"] = newL3PipelineAtReview("pipe-2")
	err = svc.Approve(context.Background(), "pipe-2", "impl", "bob", domain.GateChecklist{}, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if gateMock.events[1].PrevHash != firstHash {
		t.Errorf("second event prevHash = %q, want %q (should chain from first)", gateMock.events[1].PrevHash, firstHash)
	}
}
