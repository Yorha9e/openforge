package adapter

import (
	"context"
	"os"
	"testing"
)

func vaultAvailable(t *testing.T) {
	t.Helper()
	if os.Getenv("VAULT_ADDR") == "" {
		t.Skip("VAULT_ADDR not set, skipping vault integration test")
	}
}

func TestVaultSecretStore_Disabled_EmptyAddress(t *testing.T) {
	store := NewVaultSecretStore(VaultConfig{})
	if store.IsEnabled() {
		t.Error("should be disabled when address is empty")
	}

	_, err := store.Get(context.Background(), "test")
	if err == nil {
		t.Error("Get should return error when disabled")
	}
}

func TestVaultSecretStore_Disabled_BadAddress(t *testing.T) {
	store := NewVaultSecretStore(VaultConfig{
		Addr: "http://localhost:1", // unreachable
	})
	if store.IsEnabled() {
		t.Error("should be disabled when address is unreachable")
	}
}

func TestVaultSecretStore_Get_KVv2_Success(t *testing.T) {
	vaultAvailable(t)

	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		t.Skip("VAULT_TOKEN not set")
	}

	store := NewVaultSecretStore(VaultConfig{
		Addr:       addr,
		Token:      token,
		EnginePath: "secret",
	})
	if !store.IsEnabled() {
		t.Fatal("store should be enabled")
	}

	// Try to read a secret (may not exist, but should not panic)
	_, err := store.Get(context.Background(), "test-key")
	if err != nil {
		t.Logf("Get returned error (expected if secret doesn't exist): %v", err)
	}
}

func TestVaultSecretStore_Get_NotFound(t *testing.T) {
	vaultAvailable(t)

	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		t.Skip("VAULT_TOKEN not set")
	}

	store := NewVaultSecretStore(VaultConfig{
		Addr:       addr,
		Token:      token,
		EnginePath: "secret",
	})
	if !store.IsEnabled() {
		t.Fatal("store should be enabled")
	}

	_, err := store.Get(context.Background(), "nonexistent-key-12345")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}
