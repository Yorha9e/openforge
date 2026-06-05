package profile

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadVerifiesValidSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "test.yaml")
	content := []byte("profile: test\nsecurity_tier: dev\n")
	os.WriteFile(profilePath, content, 0644)
	sig := ed25519.Sign(priv, content)
	os.WriteFile(profilePath+".sig", sig, 0644)
	t.Setenv("OF_PROFILE_PUBKEY", hex.EncodeToString(pub))
	cfg, err := Load(profilePath, true)
	if err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
	if cfg.Profile != "test" {
		t.Errorf("expected profile=test, got %s", cfg.Profile)
	}
}

func TestLoadRejectsTamperedProfile(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "test.yaml")
	original := []byte("profile: test\nsecurity_tier: dev\n")
	os.WriteFile(profilePath, original, 0644)
	sig := ed25519.Sign(priv, original)
	os.WriteFile(profilePath, []byte("profile: test\nsecurity_tier: prod\n"), 0644)
	os.WriteFile(profilePath+".sig", sig, 0644)
	t.Setenv("OF_PROFILE_PUBKEY", hex.EncodeToString(pub))
	_, err = Load(profilePath, true)
	if err == nil {
		t.Fatal("expected tampered profile to be rejected")
	}
}

func TestLoadSkipsVerificationWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "test.yaml")
	os.WriteFile(profilePath, []byte("profile: test\nsecurity_tier: dev\n"), 0644)
	cfg, err := Load(profilePath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Profile != "test" {
		t.Errorf("expected profile=test, got %s", cfg.Profile)
	}
}
