package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/service"
	"openforge/internal/shared/profile"

	authdomain "openforge/internal/auth/domain"
)

type stubPipelineRepo struct {
	pipelines  map[string]*domain.Pipeline
	getByIDErr error
	createErr  error
}

func (s *stubPipelineRepo) Create(ctx context.Context, p *domain.Pipeline) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.pipelines[p.ID] = p
	return nil
}

func (s *stubPipelineRepo) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	p, ok := s.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	return p, nil
}

func (s *stubPipelineRepo) ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error) {
	return nil, nil
}

func (s *stubPipelineRepo) UpdateStatus(ctx context.Context, id string, status string, version int) error {
	return nil
}

func (s *stubPipelineRepo) IncrementBacktrack(ctx context.Context, id string) error {
	return nil
}

func withTestUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, authdomain.UserIDContextKey, userID)
}

func TestHandleForkPipeline_Success(t *testing.T) {
	repo := &stubPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"pipe-parent": domain.NewPipeline("pipe-parent", "proj-A", "Parent", "alice", 1, 1),
		},
	}
	of := &profile.OpenForge{
		PipelineSvc: service.NewPipelineService(repo),
	}

	body, _ := json.Marshal(map[string]string{"title": "My Fork"})
	req := httptest.NewRequest("POST", "/api/pipelines/pipe-parent/fork", bytes.NewReader(body))
	req.SetPathValue("id", "pipe-parent")
	req = req.WithContext(withTestUser(req.Context(), "bob"))
	w := httptest.NewRecorder()

	handleForkPipeline(of)(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var child domain.Pipeline
	json.Unmarshal(w.Body.Bytes(), &child)
	if child.Title != "My Fork" {
		t.Errorf("title = %q, want My Fork", child.Title)
	}
	if !child.IsSubPipeline() {
		t.Error("forked pipeline should be sub-pipeline")
	}
}

func TestHandleForkPipeline_ParentNotFound(t *testing.T) {
	repo := &stubPipelineRepo{pipelines: map[string]*domain.Pipeline{}}
	of := &profile.OpenForge{
		PipelineSvc: service.NewPipelineService(repo),
	}

	body, _ := json.Marshal(map[string]string{"title": "Ghost Fork"})
	req := httptest.NewRequest("POST", "/api/pipelines/nonexistent/fork", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	req = req.WithContext(withTestUser(req.Context(), "bob"))
	w := httptest.NewRecorder()

	handleForkPipeline(of)(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500 for not found, got %d", w.Code)
	}
}
