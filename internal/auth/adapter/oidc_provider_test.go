package adapter

import (
	"encoding/base64"
	"testing"

	"openforge/internal/shared/profile"
)

func TestParseIDTokenUnsafeRejectsMalformed(t *testing.T) {
	_, err := parseIDTokenUnsafe("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for malformed JWT")
	}
}

func TestParseIDTokenUnsafeRejectsMissingSub(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"a@b.com"}`))
	token := header + "." + payload + "." + base64.RawURLEncoding.EncodeToString([]byte("sig"))
	_, err := parseIDTokenUnsafe(token)
	if err == nil {
		t.Fatal("expected error for missing sub claim")
	}
}

func TestParseIDTokenUnsafeParsesValidClaims(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := `{"sub":"user-123","email":"test@example.com","name":"Test","groups":["dev"]}`
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	token := header + "." + payload + "." + base64.RawURLEncoding.EncodeToString([]byte("sig"))
	user, err := parseIDTokenUnsafe(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Sub != "user-123" {
		t.Errorf("expected sub=user-123, got %s", user.Sub)
	}
}

func TestNewOIDCProviderDisabled(t *testing.T) {
	p, err := NewOIDCProvider(profile.OIDCConfig{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error for disabled config: %v", err)
	}
	if p.Enabled() {
		t.Fatal("expected disabled provider")
	}
	_, err = p.AuthCodeURL("state")
	if err == nil {
		t.Fatal("expected error from disabled provider")
	}
}

func TestNewOIDCProviderRejectsInvalidConfig(t *testing.T) {
	_, err := NewOIDCProvider(profile.OIDCConfig{
		Enabled:   true,
		IssuerURL: "",
		ClientID:  "some-client",
	})
	if err == nil {
		t.Fatal("expected error for enabled config with empty issuer_url")
	}
}
