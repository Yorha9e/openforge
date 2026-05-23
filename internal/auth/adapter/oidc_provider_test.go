package adapter

import (
	"context"
	"testing"

	"openforge/internal/shared/profile"
)

func TestOIDCConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  profile.OIDCConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  profile.OIDCConfig{Enabled: true, IssuerURL: "https://auth.corp.com", ClientID: "openforge", ClientSecret: "***", RedirectURL: "https://of.corp.com/callback"},
			wantErr: false,
		},
		{
			name:    "missing issuer",
			config:  profile.OIDCConfig{Enabled: true, ClientID: "x", ClientSecret: "y", RedirectURL: "z"},
			wantErr: true,
		},
		{
			name:    "missing client_id",
			config:  profile.OIDCConfig{Enabled: true, IssuerURL: "https://x.com", ClientSecret: "y", RedirectURL: "z"},
			wantErr: true,
		},
		{
			name:    "disabled is valid even with empty fields",
			config:  profile.OIDCConfig{Enabled: false},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOIDCConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestOIDCProvider_DisabledIsNoop(t *testing.T) {
	p := NewOIDCProvider(profile.OIDCConfig{Enabled: false})
	if _, err := p.AuthCodeURL("state"); err == nil {
		t.Error("AuthCodeURL should return error when disabled")
	}
	if _, err := p.Exchange(context.TODO(), "any-code"); err == nil {		t.Error("Exchange should return error when disabled")
	}
}
