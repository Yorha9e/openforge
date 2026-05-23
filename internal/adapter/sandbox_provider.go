package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"openforge/internal/shared/kernel"
)

// SandboxProviderConfig holds pool configuration.
type SandboxProviderConfig struct {
	WarmCount   int
	MaxTotal    int
	IdleTimeout time.Duration
	Image       string
}

func defaultSandboxProviderConfig() SandboxProviderConfig {
	return SandboxProviderConfig{
		WarmCount:   10,
		MaxTotal:    30,
		IdleTimeout: 10 * time.Minute,
		Image:       "openforge/sandbox-node:latest",
	}
}

// PooledSandbox wraps a container with pool metadata.
type PooledSandbox struct {
	ID        string
	CreatedAt time.Time
	LastUsed  time.Time
}

// SandboxProvider manages a warm pool of sandbox containers.
type SandboxProvider struct {
	cfg       SandboxProviderConfig
	mu        sync.Mutex
	warm      []*PooledSandbox
	active    int
	runtime   kernel.ContainerRuntime // Phase 4+: Docker API; MVP: noop
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewSandboxProvider creates a new SandboxProvider and starts background
// goroutines for reaping idle containers and filling the warm pool.
func NewSandboxProvider(cfg SandboxProviderConfig) *SandboxProvider {
	if cfg.WarmCount <= 0 {
		cfg.WarmCount = 10
	}
	if cfg.MaxTotal <= 0 {
		cfg.MaxTotal = 30
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 10 * time.Minute
	}
	if cfg.Image == "" {
		cfg.Image = "openforge/sandbox-node:latest"
	}
	p := &SandboxProvider{
		cfg:     cfg,
		runtime: newNoopRuntime(),
		stopCh:  make(chan struct{}),
	}
	go p.reaper()
	go p.filler()
	return p
}

// Acquire retrieves a sandbox from the warm pool or cold-starts one.
func (p *SandboxProvider) Acquire(ctx context.Context) (*PooledSandbox, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Try warm pool first — skip expired entries (LRU eviction at acquire time)
	p.evictExpiredLocked(time.Now())

	if len(p.warm) > 0 {
		sb := p.warm[len(p.warm)-1]
		p.warm = p.warm[:len(p.warm)-1]
		sb.LastUsed = time.Now()
		p.active++
		return sb, nil
	}

	// Cold start: create new container
	if p.active >= p.cfg.MaxTotal {
		return nil, fmt.Errorf("sandbox pool exhausted: %d/%d active", p.active, p.cfg.MaxTotal)
	}

	id := fmt.Sprintf("sb-%d", time.Now().UnixNano())
	sb := &PooledSandbox{ID: id, CreatedAt: time.Now(), LastUsed: time.Now()}
	p.active++
	return sb, nil
}

// Release returns a sandbox to the warm pool.
func (p *SandboxProvider) Release(sb *PooledSandbox) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.active--
	sb.LastUsed = time.Now()

	// Reset and return to warm pool
	if len(p.warm) < p.cfg.WarmCount {
		p.warm = append(p.warm, sb)
	}
}

// Drain stops background goroutines and clears the pool.
func (p *SandboxProvider) Drain() {
	p.closeOnce.Do(func() { close(p.stopCh) })
	p.mu.Lock()
	defer p.mu.Unlock()
	p.warm = nil
	p.active = 0
}

// WarmCount returns current warm pool size.
func (p *SandboxProvider) WarmCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.warm)
}

// ActiveCount returns currently checked-out sandboxes.
func (p *SandboxProvider) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// reaper evicts idle containers past TTL.
func (p *SandboxProvider) reaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			p.evictExpiredLocked(time.Now())
			p.mu.Unlock()
		}
	}
}

// filler keeps warm pool at target count.
func (p *SandboxProvider) filler() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			needed := p.cfg.WarmCount - len(p.warm)
			p.mu.Unlock()
			for i := 0; i < needed; i++ {
				p.mu.Lock()
				if len(p.warm) < p.cfg.WarmCount {
					id := fmt.Sprintf("sb-%d-fill", time.Now().UnixNano())
					p.warm = append(p.warm, &PooledSandbox{ID: id, CreatedAt: time.Now(), LastUsed: time.Now()})
				}
				p.mu.Unlock()
			}
		}
	}
}

// evictExpiredLocked removes warm containers whose LastUsed is past IdleTimeout.
// Must be called with p.mu held.
func (p *SandboxProvider) evictExpiredLocked(now time.Time) {
	cutoff := now.Add(-p.cfg.IdleTimeout)
	var kept []*PooledSandbox
	for _, sb := range p.warm {
		if sb.LastUsed.After(cutoff) {
			kept = append(kept, sb)
		}
	}
	p.warm = kept
}

// noopRuntime is a placeholder until Docker SDK integration (post-Phase 4).
type noopRuntime struct{}

func newNoopRuntime() kernel.ContainerRuntime { return &noopRuntime{} }

func (r *noopRuntime) Create(ctx context.Context, spec kernel.ContainerSpec) (kernel.Container, error) {
	return kernel.Container{ID: "noop"}, nil
}
func (r *noopRuntime) Start(ctx context.Context, id string) error  { return nil }
func (r *noopRuntime) Stop(ctx context.Context, id string) error   { return nil }
func (r *noopRuntime) Remove(ctx context.Context, id string) error { return nil }
func (r *noopRuntime) List(ctx context.Context) ([]kernel.Container, error) {
	return nil, nil
}
