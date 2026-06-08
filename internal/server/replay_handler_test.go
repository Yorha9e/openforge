package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentdomain "openforge/internal/agent/domain"
	authdomain "openforge/internal/auth/domain"
	"openforge/internal/shared/featureflags"
	"openforge/internal/shared/profile"
)

func newTestOpenForge(flags *featureflags.FeatureFlags, ts agentdomain.TraceStore) *profile.OpenForge {
	if flags == nil {
		flags = featureflags.Defaults()
	}
	return &profile.OpenForge{FeatureFlags: flags, TraceStore: ts}
}

// ---- handleReplayPipeline ----

func TestReplayPipeline_ReturnsTraceEvents(t *testing.T) {
	now := time.Now()
	store := agentdomain.NewMemTraceStore()
	for i := 0; i < 3; i++ {
		_ = store.Append(context.Background(), agentdomain.TraceEvent{
			PipelineID: "p-1",
			Stage:      "plan",
			Event:      []string{"stage.start", "llm.delta", "stage.end"}[i],
			Payload:    map[string]any{"i": i},
			Timestamp:  now.Add(time.Duration(i) * time.Second),
		})
	}

	of := newTestOpenForge(nil, store)
	handler := handleReplayPipeline(of)

	req := httptest.NewRequest("GET", "/api/pipelines/p-1/replay", nil)
	req.SetPathValue("id", "p-1")
	req = req.WithContext(withAuthRole(req.Context(), "observer"))
	w := httptest.NewRecorder()
	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		PipelineID string                 `json:"pipeline_id"`
		Events     []agentdomain.TraceEvent `json:"events"`
		DurationS  float64                `json:"duration_s"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "p-1", resp.PipelineID)
	assert.Len(t, resp.Events, 3, "expected 3 events")
	assert.Equal(t, "stage.start", resp.Events[0].Event)
	assert.InDelta(t, 2.0, resp.DurationS, 0.5, "duration_s should approximate span of 3 events")
}

func TestReplayPipeline_UnauthorizedWithoutRole(t *testing.T) {
	store := agentdomain.NewMemTraceStore()
	of := newTestOpenForge(nil, store)
	handler := handleReplayPipeline(of)

	req := httptest.NewRequest("GET", "/api/pipelines/p-1/replay", nil)
	req.SetPathValue("id", "p-1")
	// no role stamped
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestReplayPipeline_ServiceUnavailableWhenStoreMissing(t *testing.T) {
	of := newTestOpenForge(nil, nil)
	handler := handleReplayPipeline(of)

	req := httptest.NewRequest("GET", "/api/pipelines/p-1/replay", nil)
	req.SetPathValue("id", "p-1")
	req = req.WithContext(withAuthRole(req.Context(), "observer"))
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- handleReplCommand ----

func TestReplCommand_RestartRequiresFeatureFlag(t *testing.T) {
	of := newTestOpenForge(featureflags.Defaults(), nil) // ProductionOps == false
	handler := handleReplCommand(of)

	body, _ := json.Marshal(replCommand{Command: "restart"})
	req := httptest.NewRequest("POST", "/api/repl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withAuthRole(req.Context(), "admin"))
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestReplCommand_UnsupportedCommand(t *testing.T) {
	ff := featureflags.Defaults()
	ff.ProductionOps = true
	of := newTestOpenForge(ff, nil)
	handler := handleReplCommand(of)

	body, _ := json.Marshal(replCommand{Command: "hack"})
	req := httptest.NewRequest("POST", "/api/repl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withAuthRole(req.Context(), "admin"))
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported command")
}

func TestReplCommand_AllowedCommandReturnsOK(t *testing.T) {
	ff := featureflags.Defaults()
	ff.ProductionOps = true
	of := newTestOpenForge(ff, nil)
	handler := handleReplCommand(of)

	for _, cmd := range []string{"restart", "pause", "re-run-stage"} {
		body, _ := json.Marshal(replCommand{Command: cmd, Args: map[string]interface{}{"pipeline_id": "p-1"}})
		req := httptest.NewRequest("POST", "/api/repl", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(withAuthRole(req.Context(), "admin"))
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "command %q should be accepted", cmd)
	}
}

// ---- helpers ----

func withAuthRole(ctx context.Context, role string) context.Context {
	ctx = context.WithValue(ctx, authdomain.UserIDContextKey, "tester")
	ctx = context.WithValue(ctx, authdomain.UserRoleContextKey, role)
	return ctx
}
