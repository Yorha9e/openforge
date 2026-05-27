package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// VaultConfig holds Vault connection parameters (decoupled from profile package).
type VaultConfig struct {
	Addr          string
	RoleID        string
	SecretID      string
	Token         string
	EnginePath    string
	EngineVersion string
	Timeout       time.Duration
}

// VaultSecretStore implements kernel.SecretStore using HashiCorp Vault.
type VaultSecretStore struct {
	client  *vault.Client
	mount   string
	timeout time.Duration
	enabled bool
	isKVv2  bool
}

// NewVaultSecretStore creates a new Vault-backed secret store.
// If the Vault address is empty or connection fails, enabled=false and system continues with noop.
func NewVaultSecretStore(cfg VaultConfig) *VaultSecretStore {
	if cfg.Addr == "" {
		slog.Warn("vault secret store disabled: empty address")
		return &VaultSecretStore{enabled: false}
	}

	config := vault.DefaultConfig()
	config.Address = cfg.Addr
	if cfg.Timeout > 0 {
		config.Timeout = cfg.Timeout
	} else {
		config.Timeout = 3 * time.Second
	}

	client, err := vault.NewClient(config)
	if err != nil {
		slog.Warn("vault client creation failed, falling back to noop", "error", err)
		return &VaultSecretStore{enabled: false}
	}

	// Authentication: Token fallback first, then AppRole
	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	} else if cfg.RoleID != "" && cfg.SecretID != "" {
		// AppRole authentication
		path := "auth/approle/login"
		data := map[string]any{
			"role_id":   cfg.RoleID,
			"secret_id": cfg.SecretID,
		}
		secret, err := client.Logical().Write(path, data)
		if err != nil {
			slog.Warn("vault approle auth failed, falling back to noop", "error", err)
			return &VaultSecretStore{enabled: false}
		}
		if secret == nil || secret.Auth == nil {
			slog.Warn("vault approle auth returned nil, falling back to noop")
			return &VaultSecretStore{enabled: false}
		}
		client.SetToken(secret.Auth.ClientToken)
	}

	// Ping to verify connectivity
	if _, err := client.Sys().Health(); err != nil {
		slog.Warn("vault ping failed, falling back to noop", "error", err)
		return &VaultSecretStore{enabled: false}
	}

	mount := cfg.EnginePath
	if mount == "" {
		mount = "secret"
	}

	// G14: Auto-detect KV engine version
	isKVv2 := true // default to v2
	switch cfg.EngineVersion {
	case "":
		// Try to detect by reading mount info
		mountInfo, err := client.Logical().Read("sys/internal/ui/mounts/" + mount)
		if err == nil && mountInfo != nil {
			if data, ok := mountInfo.Data["options"].(map[string]any); ok {
				if version, ok := data["version"].(string); ok && version == "1" {
					isKVv2 = false
					slog.Info("vault KV v1 detected", "mount", mount)
				}
			}
		}
	case "v1":
		isKVv2 = false
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	slog.Info("vault secret store enabled",
		"addr", cfg.Addr,
		"mount", mount,
		"kv_version", map[bool]string{true: "v2", false: "v1"}[isKVv2],
	)

	return &VaultSecretStore{
		client:  client,
		mount:   mount,
		timeout: timeout,
		enabled: true,
		isKVv2:  isKVv2,
	}
}

// Get retrieves a secret from Vault.
func (v *VaultSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
	if !v.enabled {
		return nil, fmt.Errorf("vault secret store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	var path string
	if v.isKVv2 {
		path = fmt.Sprintf("%s/data/%s", v.mount, key)
	} else {
		path = fmt.Sprintf("%s/%s", v.mount, key)
	}

	secret, err := v.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault read %s: %w", key, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("vault secret not found: %s", key)
	}

	// KV v2 wraps data in a "data" field
	data := secret.Data
	if v.isKVv2 {
		if inner, ok := data["data"].(map[string]any); ok {
			data = inner
		}
	}

	// Try to get "value" field first, then first string field
	if val, ok := data["value"].(string); ok {
		return []byte(val), nil
	}

	for _, val := range data {
		if str, ok := val.(string); ok {
			return []byte(str), nil
		}
	}

	return nil, fmt.Errorf("vault secret %s has no string value", key)
}

// IsEnabled returns whether the store is operational.
func (v *VaultSecretStore) IsEnabled() bool {
	return v.enabled
}
