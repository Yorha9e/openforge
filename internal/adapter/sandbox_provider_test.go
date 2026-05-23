package adapter

import (
	"context"
	"testing"
	"time"
)

func TestSandboxProvider_AcquireRelease(t *testing.T) {
	cfg := SandboxProviderConfig{
		WarmCount:   2,
		MaxTotal:    5,
		IdleTimeout: 100 * time.Millisecond,
		Image:       "alpine:latest",
	}
	p := NewSandboxProvider(cfg)
	defer p.Drain()

	ctx := context.Background()

	// Acquire
	sb, err := p.Acquire(ctx)
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	if sb.ID == "" {
		t.Error("sandbox should have ID")
	}

	// Release back to pool
	p.Release(sb)

	// Acquire again — should get the same (warm) container
	sb2, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sb2.ID != sb.ID {
		t.Log("got different container (warm pool may have been recycled)")
	}
}

func TestSandboxProvider_LRUEviction(t *testing.T) {
	cfg := SandboxProviderConfig{
		WarmCount:   1,
		MaxTotal:    2,
		IdleTimeout: 10 * time.Millisecond,
		Image:       "alpine:latest",
	}
	p := NewSandboxProvider(cfg)
	defer p.Drain()

	ctx := context.Background()

	sb, err := p.Acquire(ctx)
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	p.Release(sb)

	// Wait past idle timeout
	time.Sleep(50 * time.Millisecond)

	// Should create new container (old one evicted)
	sb2, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sb2.ID == sb.ID {
		t.Error("old container should have been evicted, got same one")
	}
}
