package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
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

type OIDCProvider struct {
	config   profile.OIDCConfig
	oauth    *oauth2.Config
	client   *http.Client
	verifier *oidc.IDTokenVerifier
}

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
	provider, err := oidc.NewProvider(context.Background(), config.IssuerURL)
	var verifier *oidc.IDTokenVerifier
	if err == nil {
		verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	}
	return &OIDCProvider{
		config:   config,
		oauth:    oauth,
		client:   &http.Client{Timeout: 10 * time.Second},
		verifier: verifier,
	}
}

func (p *OIDCProvider) AuthCodeURL(state string) (string, error) {
	if !p.config.Enabled {
		return "", fmt.Errorf("OIDC not enabled")
	}
	return p.oauth.AuthCodeURL(state), nil
}

type OIDCUser struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*OIDCUser, error) {
	if !p.config.Enabled {
		return nil, fmt.Errorf("OIDC not enabled")
	}
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}
	if p.verifier != nil {
		idToken, err := p.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			return nil, fmt.Errorf("id_token verification failed: %w", err)
		}
		var user OIDCUser
		if err := idToken.Claims(&user); err != nil {
			return nil, fmt.Errorf("parse claims: %w", err)
		}
		if user.Sub == "" {
			return nil, fmt.Errorf("id_token missing sub claim")
		}
		return &user, nil
	}
	return parseIDTokenUnsafe(rawIDToken)
}

func parseIDTokenUnsafe(raw string) (*OIDCUser, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
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

func (p *OIDCProvider) Enabled() bool { return p.config.Enabled }
