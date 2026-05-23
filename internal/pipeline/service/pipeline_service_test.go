package service

import (
	"context"
	"errors"
	"testing"

	"openforge/internal/pipeline/domain"
)

// mockPipelineRepo implements port.PipelineRepository for testing.
type mockPipelineRepo struct {
	pipelines         map[string]*domain.Pipeline
	getByIDErr        error
	updateStatusErr   error
	lastUpdateID      string
	lastUpdateStatus  string
	lastUpdateVersion int
}

func (m *mockPipelineRepo) Create(_ context.Context, p *domain.Pipeline) error {
	m.pipelines[p.ID] = p
	return nil
}

func (m *mockPipelineRepo) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	p, ok := m.pipelines[id]
	if !ok {
		return nil, errors.New("pipeline not found")
	}
	return p, nil
}

func (m *mockPipelineRepo) ListByProject(_ context.Context, projectID string) ([]*domain.Pipeline, error) {
	var result []*domain.Pipeline
	for _, p := range m.pipelines {
		if p.ProjectID == projectID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPipelineRepo) UpdateStatus(_ context.Context, id, status string, version int) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	m.lastUpdateID = id
	m.lastUpdateStatus = status
	m.lastUpdateVersion = version
	p, ok := m.pipelines[id]
	if ok {
		p.Status = status
	}
	return nil
}

func (m *mockPipelineRepo) IncrementBacktrack(_ context.Context, id string) error {
	return nil
}

func newL3Pipeline(id string) *domain.Pipeline {
	return &domain.Pipeline{
		ID:           id,
		ProjectID:    "proj1",
		Title:        "Test",
		Level:        "L3",
		Status:       "pending",
		CurrentStage: "clarify",
		Version:      1,
		Stages: []domain.Stage{
			{Type: "clarify", Status: "pending"},
			{Type: "decompose", Status: "pending"},
			{Type: "impl", Status: "pending"},
			{Type: "test", Status: "pending"},
			{Type: "deploy", Status: "pending"},
			{Type: "verify", Status: "pending"},
		},
	}
}

func newL1Pipeline(id string) *domain.Pipeline {
	return &domain.Pipeline{
		ID:           id,
		ProjectID:    "proj1",
		Title:        "Test",
		Level:        "L1",
		Status:       "running",
		CurrentStage: "clarify",
		Version:      1,
		Stages: []domain.Stage{
			{Type: "clarify", Status: "running"},
			{Type: "impl", Status: "pending"},
			{Type: "test", Status: "pending"},
			{Type: "deploy", Status: "pending"},
			{Type: "verify", Status: "pending"},
		},
	}
}

func TestPipelineService_Start(t *testing.T) {
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"p1": newL3Pipeline("p1"),
		},
	}
	svc := NewPipelineService(mock)

	err := svc.Start(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}

	p := mock.pipelines["p1"]
	if p.Status != "running" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "running")
	}
	if p.Stages[0].Status != "running" {
		t.Errorf("first stage status = %q, want %q", p.Stages[0].Status, "running")
	}
	if mock.lastUpdateStatus != "running" {
		t.Errorf("UpdateStatus called with status %q, want %q", mock.lastUpdateStatus, "running")
	}
}

func TestPipelineService_Start_InvalidTransition(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "completed"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Start(context.Background(), "p1")
	if err == nil {
		t.Fatal("Start() expected error for completed pipeline")
	}
}

func TestPipelineService_Start_RepoError(t *testing.T) {
	mock := &mockPipelineRepo{
		getByIDErr: errors.New("db unavailable"),
	}
	svc := NewPipelineService(mock)

	err := svc.Start(context.Background(), "p1")
	if err == nil {
		t.Fatal("Start() expected error when repo fails")
	}
}

func TestPipelineService_AdvanceStage_NeedsGate(t *testing.T) {
	p := &domain.Pipeline{
		ID:           "p1",
		ProjectID:    "proj1",
		Title:        "Test",
		Level:        "L3",
		Status:       "running",
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
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.AdvanceStage(context.Background(), "p1")
	if err != nil {
		t.Fatalf("AdvanceStage() unexpected error: %v", err)
	}

	if p.Status != "awaiting_review" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "awaiting_review")
	}
	if mock.lastUpdateStatus != "awaiting_review" {
		t.Errorf("UpdateStatus called with %q, want %q", mock.lastUpdateStatus, "awaiting_review")
	}
}

func TestPipelineService_AdvanceStage_NoGate(t *testing.T) {
	p := newL1Pipeline("p1")
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.AdvanceStage(context.Background(), "p1")
	if err != nil {
		t.Fatalf("AdvanceStage() unexpected error: %v", err)
	}

	if p.Status != "running" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "running")
	}
	if p.CurrentStage != "impl" {
		t.Errorf("current stage = %q, want %q", p.CurrentStage, "impl")
	}
	if p.Stages[0].Status != "passed" {
		t.Errorf("first stage status = %q, want %q", p.Stages[0].Status, "passed")
	}
}

func TestPipelineService_AdvanceStage_RepoError(t *testing.T) {
	mock := &mockPipelineRepo{
		getByIDErr: errors.New("db unavailable"),
	}
	svc := NewPipelineService(mock)

	err := svc.AdvanceStage(context.Background(), "p1")
	if err == nil {
		t.Fatal("AdvanceStage() expected error when repo fails")
	}
}

func TestPipelineService_Pause(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "running"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Pause(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Pause() unexpected error: %v", err)
	}

	if p.Status != "paused" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "paused")
	}
	if mock.lastUpdateStatus != "paused" {
		t.Errorf("UpdateStatus called with %q, want %q", mock.lastUpdateStatus, "paused")
	}
}

func TestPipelineService_Pause_InvalidState(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "pending"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Pause(context.Background(), "p1")
	if err == nil {
		t.Fatal("Pause() expected error when pipeline is pending")
	}
}

func TestPipelineService_Resume(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "paused"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Resume(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Resume() unexpected error: %v", err)
	}

	if p.Status != "running" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "running")
	}
	if mock.lastUpdateStatus != "running" {
		t.Errorf("UpdateStatus called with %q, want %q", mock.lastUpdateStatus, "running")
	}
}

func TestPipelineService_Resume_InvalidState(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "pending"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Resume(context.Background(), "p1")
	if err == nil {
		t.Fatal("Resume() expected error when pipeline is pending")
	}
}

func TestPipelineService_Cancel(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "running"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Cancel(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Cancel() unexpected error: %v", err)
	}

	if p.Status != "cancelled" {
		t.Errorf("pipeline status = %q, want %q", p.Status, "cancelled")
	}
	if mock.lastUpdateStatus != "cancelled" {
		t.Errorf("UpdateStatus called with %q, want %q", mock.lastUpdateStatus, "cancelled")
	}
}

func TestPipelineService_Cancel_InvalidState(t *testing.T) {
	p := newL3Pipeline("p1")
	p.Status = "completed"
	mock := &mockPipelineRepo{
		pipelines: map[string]*domain.Pipeline{"p1": p},
	}
	svc := NewPipelineService(mock)

	err := svc.Cancel(context.Background(), "p1")
	if err == nil {
		t.Fatal("Cancel() expected error when pipeline is completed")
	}
}

func TestPipelineService_Cancel_RepoError(t *testing.T) {
	mock := &mockPipelineRepo{
		getByIDErr: errors.New("db unavailable"),
	}
	svc := NewPipelineService(mock)

	err := svc.Cancel(context.Background(), "p1")
	if err == nil {
		t.Fatal("Cancel() expected error when repo fails")
	}
}

func TestPipelineService_Resume_RepoError(t *testing.T) {
	mock := &mockPipelineRepo{
		getByIDErr: errors.New("db unavailable"),
	}
	svc := NewPipelineService(mock)

	err := svc.Resume(context.Background(), "p1")
	if err == nil {
		t.Fatal("Resume() expected error when repo fails")
	}
}
