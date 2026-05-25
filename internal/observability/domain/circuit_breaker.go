package domain

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state machine.
type State int

const (
	StateClosed   State = iota // normal operation
	StateOpen                   // rejecting requests
	StateHalfOpen               // probing for recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// BreakerConfig defines per-dependency threshold and timing parameters.
type BreakerConfig struct {
	Name            string
	MaxFailures     int
	OpenDuration    time.Duration
	HalfOpenMaxReqs int
}

// Default configurations for external dependencies.
var (
	DefaultBreakerLLM = BreakerConfig{
		Name: "llm", MaxFailures: 5,
		OpenDuration: 120 * time.Second, HalfOpenMaxReqs: 1,
	}
	DefaultBreakerDocker = BreakerConfig{
		Name: "docker", MaxFailures: 3,
		OpenDuration: 60 * time.Second, HalfOpenMaxReqs: 1,
	}
	DefaultBreakerMinIO = BreakerConfig{
		Name: "minio", MaxFailures: 3,
		OpenDuration: 60 * time.Second, HalfOpenMaxReqs: 1,
	}
	DefaultBreakerPostgres = BreakerConfig{
		Name: "postgres", MaxFailures: 3,
		OpenDuration: 10 * time.Second, HalfOpenMaxReqs: 1,
	}
)

// ErrCircuitOpen is returned when the breaker rejects a call.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Breaker implements the circuit breaker pattern with CLOSED, OPEN, and
// HALF_OPEN states. It is safe for concurrent use.
type Breaker struct {
	config       BreakerConfig
	state        State
	failures     int
	lastFailTime time.Time
	openedAt     time.Time
	halfOpenReqs int
	mu           sync.Mutex
}

// NewBreaker creates a circuit breaker starting in the CLOSED state.
func NewBreaker(config BreakerConfig) *Breaker {
	return &Breaker{config: config, state: StateClosed}
}

// Call executes fn under circuit breaker protection.  It returns
// ErrCircuitOpen when the breaker is OPEN (and the open duration has not yet
// elapsed) or when the HALF_OPEN probe limit is reached.
//
// On success the failure count resets; on failure it increments and may
// transition the breaker to OPEN.
func (b *Breaker) Call(fn func() error) error {
	b.mu.Lock()
	switch b.state {
	case StateOpen:
		if time.Since(b.openedAt) >= b.config.OpenDuration {
			b.state = StateHalfOpen
			b.halfOpenReqs = 0
		} else {
			b.mu.Unlock()
			return ErrCircuitOpen
		}
	case StateHalfOpen:
		if b.halfOpenReqs >= b.config.HalfOpenMaxReqs {
			b.mu.Unlock()
			return ErrCircuitOpen
		}
		b.halfOpenReqs++
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.failures++
		b.lastFailTime = time.Now()
		if b.state == StateHalfOpen {
			b.state = StateOpen
			b.openedAt = time.Now()
		} else if b.failures >= b.config.MaxFailures {
			b.state = StateOpen
			b.openedAt = time.Now()
		}
		return err
	}
	b.failures = 0
	if b.state == StateHalfOpen {
		b.state = StateClosed
	}
	return nil
}

// State returns the current breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Counters returns the current failure count and last failure time for
// observability / metrics reporting.
func (b *Breaker) Counters() (failures int, lastFailTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures, b.lastFailTime
}

// --- BreakerPool ---

// BreakerPool manages a collection of named circuit breakers.
type BreakerPool struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
	defaults BreakerConfig
}

// NewBreakerPool creates a pool that applies defaults to lazily-created
// breakers.
func NewBreakerPool(defaults BreakerConfig) *BreakerPool {
	return &BreakerPool{
		breakers: make(map[string]*Breaker),
		defaults: defaults,
	}
}

// Get returns (or creates) the named breaker.  The first call for a given
// name creates it using the pool defaults.
func (p *BreakerPool) Get(name string) *Breaker {
	p.mu.RLock()
	b, ok := p.breakers[name]
	p.mu.RUnlock()
	if ok {
		return b
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Double-check after acquiring write lock.
	if b, ok := p.breakers[name]; ok {
		return b
	}
	b = NewBreaker(BreakerConfig{
		Name:            name,
		MaxFailures:     p.defaults.MaxFailures,
		OpenDuration:    p.defaults.OpenDuration,
		HalfOpenMaxReqs: p.defaults.HalfOpenMaxReqs,
	})
	p.breakers[name] = b
	return b
}

// All returns a snapshot of every registered breaker and its current state.
func (p *BreakerPool) All() map[string]State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]State, len(p.breakers))
	for name, b := range p.breakers {
		result[name] = b.State()
	}
	return result
}
