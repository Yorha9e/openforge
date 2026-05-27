package service

import (
	"sync"

	"openforge/internal/policy/domain"
)

type CanaryService struct {
	mu      sync.RWMutex
	configs []*domain.CanaryConfig
	engine  *domain.CanaryEngine
}

func NewCanaryService(configs ...*domain.CanaryConfig) *CanaryService {
	s := &CanaryService{
		configs: configs,
		engine:  domain.NewCanaryEngine(configs),
	}
	return s
}

func (s *CanaryService) Evaluate(pipelineID, projectID string, current, baseline float64, sampleSize int) []domain.EvaluateResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine.Evaluate(pipelineID, projectID, current, baseline, sampleSize)
}

func (s *CanaryService) Configs() []*domain.CanaryConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configs
}

func (s *CanaryService) Replace(configs []*domain.CanaryConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs = configs
	s.engine = domain.NewCanaryEngine(configs)
}
