package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"
)

func TestChatWS_UpgradesWithoutHTTPAuth(t *testing.T) {
	// WS route no longer uses HTTP auth middleware.
	// Connection upgrades regardless; auth happens via first-frame protocol.
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	// httptest doesn't support hijacking, so we get an error, not 401
	if rec.Code == 401 {
		t.Errorf("WS handler should not return HTTP 401, auth is via first-frame; got %d", rec.Code)
	}
}

func TestChatWS_NoHTTPAuthRequired(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	// No Authorization header at all
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	if rec.Code == 401 {
		t.Errorf("WS upgrade should not be blocked by missing HTTP auth; got 401")
	}
}
