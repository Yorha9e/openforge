package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"openforge/internal/auth/service"
	"openforge/internal/shared/profile"
)

func TestChatWS_RequiresAuth(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	rec := httptest.NewRecorder()
	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)
	if rec.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestChatWS_InvalidToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()
	handler := handleChatWS(&profile.OpenForge{}, jwtSvc)
	handler(rec, req)
	if rec.Code != 401 {
		t.Errorf("expected 401 with invalid token, got %d", rec.Code)
	}
}

func TestChatWS_AcceptsValidToken(t *testing.T) {
	jwtSvc := service.NewJWTService("test-secret", 1*time.Hour, 24*time.Hour)
	token, _ := jwtSvc.Issue("user@test.com", "pm", "")
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()
	// Wrap with auth middleware to match production wiring in routes.go.
	handler := AuthMiddleware(jwtSvc)(handleChatWS(&profile.OpenForge{}, jwtSvc))
	handler(rec, req)
	if rec.Code == 401 {
		t.Errorf("valid token should not return 401, got %d", rec.Code)
	}
}
