package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"
)

func TestChatWSRejectsMissingToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	if rec.Code == 101 {
		t.Fatalf("missing token must not upgrade")
	}
	if rec.Code != 401 {
		t.Fatalf("expected 401 for missing token, got %d", rec.Code)
	}
}

func TestChatWSRejectsInvalidToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	rec := httptest.NewRecorder()

	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)

	if rec.Code == 101 {
		t.Fatalf("invalid token must not upgrade")
	}
	if rec.Code != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
	}
}
