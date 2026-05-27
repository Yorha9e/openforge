package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	observabilitydomain "openforge/internal/observability/domain"
)

type staticResourceSnapshotProvider struct {
	snap observabilitydomain.ResourceSnapshot
}

func (p *staticResourceSnapshotProvider) Snapshot() observabilitydomain.ResourceSnapshot {
	return p.snap
}

func TestLoadShedMiddleware_AllowsNormal(t *testing.T) {
	ls := observabilitydomain.NewLoadShedder()
	// GoroutinesAvail/GoroutinesMax = 90% (Normal capacity)
	provider := &staticResourceSnapshotProvider{
		snap: observabilitydomain.ResourceSnapshot{
			GoroutinesAvail:   90,
			GoroutinesMax:     100,
			SandboxWarm:       10,
			SandboxMin:        10,
			PGIdleConns:       10,
			LLMQueueDepth:     1,
			LLMQueueThreshold: 10,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	middleware := LoadShedMiddleware(ls, provider, handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLoadShedMiddleware_RejectsCriticalNonP0(t *testing.T) {
	ls := observabilitydomain.NewLoadShedder()
	// GoroutinesAvail/GoroutinesMax = 5% (Critical capacity < 10%)
	provider := &staticResourceSnapshotProvider{
		snap: observabilitydomain.ResourceSnapshot{
			GoroutinesAvail:   5,
			GoroutinesMax:     100,
			SandboxWarm:       10,
			SandboxMin:        10,
			PGIdleConns:       10,
			LLMQueueDepth:     1,
			LLMQueueThreshold: 10,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoadShedMiddleware(ls, provider, handler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-OpenForge-Priority", "3") // Non P0 (P0 is 0)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestLoadShedMiddleware_SetsRetryAfter(t *testing.T) {
	ls := observabilitydomain.NewLoadShedder()
	provider := &staticResourceSnapshotProvider{
		snap: observabilitydomain.ResourceSnapshot{
			GoroutinesAvail:   5,
			GoroutinesMax:     100,
			SandboxWarm:       10,
			SandboxMin:        10,
			PGIdleConns:       10,
			LLMQueueDepth:     1,
			LLMQueueThreshold: 10,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoadShedMiddleware(ls, provider, handler)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header to be set")
	}
	if retryAfter != "30" { // CRITICAL corresponds to 30 seconds
		t.Fatalf("expected Retry-After to be 30, got %s", retryAfter)
	}
}
