package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"openforge/internal/shared/profile"
)

func validateOIDCConfig(c profile.OIDCConfig) error {
	if !c.Enabled {
		return nil
	}
	if c.IssuerURL == "" {
		return fmt.Errorf("issuer_url is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}
	return nil
}

// OIDCProvider handles OpenID Connect authentication flow.
type OIDCProvider struct {
	config profile.OIDCConfig
	oauth  *oauth2.Config
	client *http.Client
}

// NewOIDCProvider creates an OIDC provider. Returns a disabled no-op if config.Enabled is false.
func NewOIDCProvider(config profile.OIDCConfig) *OIDCProvider {
	if !config.Enabled {
		return &OIDCProvider{config: config}
	}
	oauth := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.IssuerURL + "/authorize",
			TokenURL: config.IssuerURL + "/token",
		},
		Scopes: append([]string{"openid", "profile", "email"}, config.Scopes...),
	}
	return &OIDCProvider{config: config, oauth: oauth, client: &http.Client{Timeout: 10 * time.Second}}
}

// AuthCodeURL returns the OIDC provider's authorization URL.
func (p *OIDCProvider) AuthCodeURL(state string) (string, error) {
	if !p.config.Enabled {
		return "", fmt.Errorf("OIDC not enabled")
	}
	return p.oauth.AuthCodeURL(state), nil
}

// OIDCUser represents an authenticated OIDC user.
type OIDCUser struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// Exchange exchanges an authorization code for an OIDC user.
// It parses the actual id_token JWT payload (fixes the Phase 6 v1 no-op bug).
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*OIDCUser, error) {
	if !p.config.Enabled {
		return nil, fmt.Errorf("OIDC not enabled")
	}
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}
	return parseIDToken(idToken)
}

// parseIDToken decodes the JWT payload without signature verification.
// MVP: trust the OIDC provider's TLS + network isolation.
// Phase 7: switch to coreos/go-oidc for full verification.
func parseIDToken(raw string) (*OIDCUser, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var user OIDCUser
	if err := json.Unmarshal(payload, &user); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	if user.Sub == "" {
		return nil, fmt.Errorf("id_token missing sub claim")
	}
	return &user, nil
}

// Enabled returns true if OIDC is configured and enabled.
func (p *OIDCProvider) Enabled() bool { return p.config.Enabled }
