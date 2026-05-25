package domain

import (
	"context"
	"math"
	"sync"
	"time"
)

// ABExperiment represents an A/B test for comparing knowledge/trajectory variants.
type ABExperiment struct {
	ID           string
	KnowledgeID  string
	CohortARatio float64
	Status       string // "running", "completed"
	Verdict      string // "promoted", "invalid", "harmful", or ""
	PValue       float64
	EffectSize   float64
	StartedAt    time.Time
	CompletedAt  time.Time
}

// ABExperimentAssignment records which cohort a pipeline was assigned to.
type ABExperimentAssignment struct {
	ID           string
	ExperimentID string
	PipelineID   string
	Cohort       string // "A" or "B"
}

// ExperimentStore persists A/B experiments and assignments.
type ExperimentStore interface {
	Create(ctx context.Context, exp *ABExperiment) error
	Get(ctx context.Context, id string) (*ABExperiment, error)
	Assign(ctx context.Context, a *ABExperimentAssignment) error
	Complete(ctx context.Context, id, verdict string, pValue, effectSize float64) error
	ListActive(ctx context.Context) ([]*ABExperiment, error)
}

// MemExperimentStore is an in-memory implementation of ExperimentStore.
type MemExperimentStore struct {
	mu          sync.RWMutex
	experiments map[string]*ABExperiment
	assignments []*ABExperimentAssignment
}

func NewMemExperimentStore() *MemExperimentStore {
	return &MemExperimentStore{
		experiments: make(map[string]*ABExperiment),
	}
}

func (s *MemExperimentStore) Create(_ context.Context, exp *ABExperiment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.experiments[exp.ID] = exp
	return nil
}

func (s *MemExperimentStore) Get(_ context.Context, id string) (*ABExperiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.experiments[id]
	if !ok {
		return nil, nil
	}
	return exp, nil
}

func (s *MemExperimentStore) Assign(_ context.Context, a *ABExperimentAssignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments = append(s.assignments, a)
	return nil
}

func (s *MemExperimentStore) Complete(_ context.Context, id, verdict string, pValue, effectSize float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.experiments[id]
	if !ok {
		return nil
	}
	exp.Status = "completed"
	exp.Verdict = verdict
	exp.PValue = pValue
	exp.EffectSize = effectSize
	exp.CompletedAt = time.Now().UTC()
	return nil
}

func (s *MemExperimentStore) ListActive(_ context.Context) ([]*ABExperiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []*ABExperiment
	for _, exp := range s.experiments {
		if exp.Status == "running" {
			active = append(active, exp)
		}
	}
	return active, nil
}

// SimpleTTest performs a simplified two-sample t-test comparing control and treatment means.
// Returns the p-value (normal approximation) and mean difference effect size.
func SimpleTTest(control, treatment []float64) (pValue, effectSize float64) {
	n1, n2 := len(control), len(treatment)
	if n1 < 2 || n2 < 2 {
		return 1.0, 0
	}

	m1, m2 := mean(control), mean(treatment)
	effectSize = m2 - m1

	v1, v2 := variance(control, m1), variance(treatment, m2)
	se := math.Sqrt(v1/float64(n1) + v2/float64(n2))
	if se == 0 {
		if effectSize == 0 {
			return 1.0, 0
		}
		return 0, effectSize // zero variance, certain difference
	}

	tStat := effectSize / se

	// Two-tailed p-value via normal approximation: erfc(|t| / sqrt(2))
	pValue = math.Erfc(math.Abs(tStat) / math.Sqrt2)
	return pValue, effectSize
}

func mean(vals []float64) float64 {
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func variance(vals []float64, m float64) float64 {
	var sum float64
	for _, v := range vals {
		d := v - m
		sum += d * d
	}
	return sum / float64(len(vals)-1)
}

// DetermineVerdict returns the experiment verdict based on statistical significance and effect size.
func DetermineVerdict(pValue, effectSize float64) string {
	const alpha = 0.05
	sig := pValue < alpha
	switch {
	case sig && effectSize > 0:
		return "promoted"
	case sig && effectSize < 0:
		return "harmful"
	case effectSize <= 0:
		return "invalid"
	default:
		return ""
	}
}
