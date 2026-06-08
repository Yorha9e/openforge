package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

func TestSecurityHeadersMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	SecurityHeaders(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("Content-Security-Policy header is required")
	} else {
		requiredDirectives := []string{
			"default-src 'self'",
			"script-src 'self'",
			"object-src 'none'",
			"base-uri 'self'",
			"frame-ancestors 'none'",
		}
		for _, directive := range requiredDirectives {
			if !strings.Contains(got, directive) {
				t.Fatalf("Content-Security-Policy = %q, missing %q", got, directive)
			}
		}
		if strings.Contains(got, "'unsafe-eval'") {
			t.Fatalf("Content-Security-Policy must not allow unsafe-eval: %q", got)
		}
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got == "" {
		t.Fatal("Referrer-Policy header is required")
	}
}

func TestRateLimitMiddleware_IPAndProject(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := RateLimitMiddleware(100 /*ipPerSec*/, 50 /*projectPerSec*/)(next)

	// IP 维度：50 个不同 IP 的请求不会互相触发限制（远低于 ipPerSec=100）
	for i := 0; i < 50; i++ {
		r := httptest.NewRequest("GET", "/x", nil)
		r.RemoteAddr = fmt.Sprintf("1.2.3.%d:80", i)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		require.Equal(t, 200, rr.Code)
	}

	// project 维度：同 IP 不同 project 50 个请求全通；第 51 个 429
	r := httptest.NewRequest("GET", "/x", nil)
	r.RemoteAddr = "5.6.7.8:80"
	r.Header.Set("X-Project-ID", "p-1")
	for i := 0; i < 50; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		require.Equal(t, 200, rr.Code)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	require.Equal(t, 429, rr.Code)
	require.Equal(t, "60", rr.Header().Get("Retry-After")) // WARNING 水位
}
