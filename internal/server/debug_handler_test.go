package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentdomain "openforge/internal/agent/domain"
	authdomain "openforge/internal/auth/domain"
	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"
)

// withAuthCtx stamps user id + role into the request context, simulating
// the post-AuthMiddleware state. This bypasses JWT signing for unit tests.
func withAuthCtx(r *http.Request, userID, role string) *http.Request {
	ctx := context.WithValue(r.Context(), authdomain.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, authdomain.UserRoleContextKey, role)
	return r.WithContext(ctx)
}

func seedTrace(t *testing.T, store *agentdomain.MemTraceStore, pipelineID string, n int) {
	t.Helper()
	now := time.Now()
	for i := range n {
		ev := agentdomain.TraceEvent{
			PipelineID: pipelineID,
			Stage:      "implementation",
			Event:      "llm_call_start",
			Payload:    map[string]any{"round": i},
			Timestamp:  now.Add(-time.Duration(n-i) * time.Minute),
		}
		if err := store.Append(context.Background(), ev); err != nil {
			t.Fatalf("seed Append: %v", err)
		}
	}
}

// --- Test 1: auth gating (401 for missing token, 200 for admin) ---

func TestDebugTraceHandler_AuthGating(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	store := agentdomain.NewMemTraceStore()
	seedTrace(t, store, "pipe-1", 1)
	of := &profile.OpenForge{TraceStore: store}

	t.Run("no_auth_header_401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/debug/trace/pipe-1", nil)
		rec := httptest.NewRecorder()
		handler := handleGetDebugTrace(of, jwtSvc)
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (body=%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("admin_with_token_200", func(t *testing.T) {
		pair, err := jwtSvc.Issue("carol", "admin", "")
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		req := httptest.NewRequest("GET", "/api/debug/trace/pipe-1", nil)
		req.SetPathValue("id", "pipe-1")
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		req = withAuthCtx(req, "carol", "admin")
		rec := httptest.NewRecorder()
		handler := handleGetDebugTrace(of, jwtSvc)
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

// --- Test 2: ?since=<RFC3339> filters old events out ---

func TestDebugTraceHandler_ReturnsEventsSinceTimestamp(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	store := agentdomain.NewMemTraceStore()
	seedTrace(t, store, "pipe-2", 3)
	of := &profile.OpenForge{TraceStore: store}

	pair, err := jwtSvc.Issue("carol", "admin", "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	since := time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/debug/trace/pipe-2?since="+since, nil)
	req.SetPathValue("id", "pipe-2")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = withAuthCtx(req, "carol", "admin")
	rec := httptest.NewRecorder()
	handler := handleGetDebugTrace(of, jwtSvc)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got []agentdomain.TraceEvent
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("event count = %d, want 1 (body=%s)", len(got), rec.Body.String())
	}
	if got[0].PipelineID != "pipe-2" || got[0].Event != "llm_call_start" {
		t.Fatalf("unexpected event: %+v", got[0])
	}
}

// --- Test 3: 30-day default when no since is supplied ---

func TestDebugTraceHandler_DefaultsTo30Days(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	store := agentdomain.NewMemTraceStore()
	now := time.Now()
	for i, ts := range []time.Time{now, now.Add(-10 * 24 * time.Hour), now.Add(-40 * 24 * time.Hour)} {
		_ = store.Append(context.Background(), agentdomain.TraceEvent{
			PipelineID: "pipe-3",
			Stage:      "implementation",
			Event:      "llm_call_start",
			Payload:    map[string]any{"i": i},
			Timestamp:  ts,
		})
	}
	of := &profile.OpenForge{TraceStore: store}

	pair, err := jwtSvc.Issue("carol", "admin", "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/debug/trace/pipe-3", nil)
	req.SetPathValue("id", "pipe-3")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = withAuthCtx(req, "carol", "admin")
	rec := httptest.NewRecorder()
	handler := handleGetDebugTrace(of, jwtSvc)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got []agentdomain.TraceEvent
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("event count = %d, want 2 (events=%v)", len(got), got)
	}
	for _, ev := range got {
		if time.Since(ev.Timestamp) > 31*24*time.Hour {
			t.Fatalf("event older than 30d leaked through default window: %+v", ev)
		}
	}
}

// --- Test 4: malformed since returns 400 ---

func TestDebugTraceHandler_BadSinceReturns400(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	store := agentdomain.NewMemTraceStore()
	of := &profile.OpenForge{TraceStore: store}

	pair, err := jwtSvc.Issue("carol", "admin", "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/debug/trace/pipe-x?since=not-a-date", nil)
	req.SetPathValue("id", "pipe-x")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = withAuthCtx(req, "carol", "admin")
	rec := httptest.NewRecorder()
	handler := handleGetDebugTrace(of, jwtSvc)
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

// --- Test 5: non-admin role is rejected with 403 ---

func TestDebugTraceHandler_NonAdminForbidden(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	store := agentdomain.NewMemTraceStore()
	seedTrace(t, store, "pipe-1", 1)
	of := &profile.OpenForge{TraceStore: store}

	pair, err := jwtSvc.Issue("alice", "dev", "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/debug/trace/pipe-1", nil)
	req.SetPathValue("id", "pipe-1")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = withAuthCtx(req, "alice", "dev")
	rec := httptest.NewRecorder()
	handler := handleGetDebugTrace(of, jwtSvc)
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", rec.Code, rec.Body.String())
	}
}

// --- Test 6: nil trace store returns 503 ---

func TestDebugTraceHandler_NilTraceStoreReturns503(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", time.Hour, time.Hour)
	of := &profile.OpenForge{}

	pair, err := jwtSvc.Issue("carol", "admin", "")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/debug/trace/pipe-x", nil)
	req.SetPathValue("id", "pipe-x")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = withAuthCtx(req, "carol", "admin")
	rec := httptest.NewRecorder()
	handler := handleGetDebugTrace(of, jwtSvc)
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
}
