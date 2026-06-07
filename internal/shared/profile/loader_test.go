package profile

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadMinimalProfile(t *testing.T) {
	tmp := t.TempDir()
	content := `
profile: minimal
security_tier: dev
secret_store: envfile
container_runtime: docker
object_store: localfs
task_queue: pg-skip-locked
event_bus: goroutine-chan
cache: memory
telemetry: stdout
service_registry: static
disaster_recovery: local-backup
load_balancer: none
notifier: stdout
database:
  host: localhost
  port: 5432
  user: test
  password: test
  dbname: test
  sslmode: disable
llm:
  default_provider: anthropic
  default_model: claude-sonnet-4-6
grpc:
  nodejs_io_addr: localhost:50051
  coordinator_addr: localhost:50052
`
	path := filepath.Join(tmp, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "minimal" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "minimal")
	}
	if cfg.SecretStore != "envfile" {
		t.Errorf("SecretStore = %q, want %q", cfg.SecretStore, "envfile")
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.LLM.DefaultProvider != "anthropic" {
		t.Errorf("LLM.DefaultProvider = %q, want %q", cfg.LLM.DefaultProvider, "anthropic")
	}
	if cfg.GRPC.NodejsIOAddr != "localhost:50051" {
		t.Errorf("GRPC.NodejsIOAddr = %q, want %q", cfg.GRPC.NodejsIOAddr, "localhost:50051")
	}
}

func TestLoadStandardProfile(t *testing.T) {
	tmp := t.TempDir()
	content := `
profile: standard
security_tier: prod
secret_store: vault-sidecar
container_runtime: docker-api
object_store: minio-single
task_queue: redis-streams
event_bus: redis-pubsub
cache: redis-single
telemetry: prometheus
service_registry: dns-srv
disaster_recovery: pg-standby
load_balancer: nginx
notifier: feishu-webhook
command_executor: docker-sandbox
database:
  host: of-pg-primary.internal
  port: 5432
  user: openforge
  dbname: openforge
  sslmode: require
`
	path := filepath.Join(tmp, "standard.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "standard" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "standard")
	}
	if cfg.SecurityTier != "prod" {
		t.Errorf("SecurityTier = %q, want %q", cfg.SecurityTier, "prod")
	}
	if cfg.TaskQueue != "redis-streams" {
		t.Errorf("TaskQueue = %q, want %q", cfg.TaskQueue, "redis-streams")
	}
	if cfg.CommandExecutor != "docker-sandbox" {
		t.Errorf("CommandExecutor = %q, want %q", cfg.CommandExecutor, "docker-sandbox")
	}
}

func TestLoadEnterpriseProfile(t *testing.T) {
	tmp := t.TempDir()
	content := `
profile: enterprise
security_tier: regulated
secret_store: vault-ha
container_runtime: k8s-pod
object_store: minio-cluster
task_queue: redis-cluster-streams
event_bus: redis-cluster-pubsub
cache: redis-cluster
telemetry: otel-collector
service_registry: k8s-service
disaster_recovery: multi-region
load_balancer: k8s-ingress
notifier: multi-channel
database:
  host: of-pg-primary.internal
  port: 5432
  user: openforge
  dbname: openforge
  sslmode: require
`
	path := filepath.Join(tmp, "enterprise.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path, false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "enterprise" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "enterprise")
	}
	if cfg.SecurityTier != "regulated" {
		t.Errorf("SecurityTier = %q, want %q", cfg.SecurityTier, "regulated")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml", false)
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "invalid.yaml")
	content := `profile: minimal\ninvalid_yaml: [`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path, false)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

func TestLoadMissingProfileField(t *testing.T) {
	tmp := t.TempDir()
	content := `
security_tier: dev
secret_store: envfile
`
	path := filepath.Join(tmp, "no_profile.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path, false)
	if err == nil {
		t.Fatal("Expected error for missing profile name, got nil")
	}
}

func TestLoadUnknownSecurityTier(t *testing.T) {
	tmp := t.TempDir()
	content := `
profile: minimal
security_tier: unknown_tier
secret_store: envfile
`
	path := filepath.Join(tmp, "bad_security.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path, false)
	if err == nil {
		t.Fatal("Expected error for unknown security tier, got nil")
	}
}

func TestLoadVerifySignature(t *testing.T) {
	tmp := t.TempDir()
	content := `
profile: minimal
security_tier: dev
secret_store: envfile
`
	path := filepath.Join(tmp, "signed.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// verifySignature=true but no OF_PROFILE_PUBKEY set and no .sig file
	_, err := Load(path, true)
	if err == nil {
		t.Fatal("Expected error when verifySignature=true without pubkey, got nil")
	}
}

// TestProfilePeriodicRevalidation_DetectsTampering verifies that the periodic
// revalidation ticker fires and surfaces a verification error when the on-disk
// profile.yaml is tampered with after loading.
func TestProfilePeriodicRevalidation_DetectsTampering(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	original := []byte("profile: test\nsecurity_tier: dev\n")
	if err := os.WriteFile(profilePath, original, 0644); err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, original)
	if err := os.WriteFile(profilePath+".sig", sig, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OF_PROFILE_PUBKEY", hex.EncodeToString(pub))

	cfg, err := Load(profilePath, true)
	if err != nil {
		t.Fatalf("initial Load() with valid signature should succeed, got: %v", err)
	}
	if cfg.path == "" {
		t.Fatal("Load() should record the profile path on Config for revalidation")
	}

	// Tamper with the on-disk yaml after loading.
	if err := os.WriteFile(profilePath, []byte("profile: tampered\nsecurity_tier: prod\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture revalidation outcomes: ok count + err count, plus the latest error.
	var (
		okCount  atomic.Int32
		errCount atomic.Int32
		mu       sync.Mutex
		lastErr  error
	)
	observer := func(path string, err error) {
		mu.Lock()
		lastErr = err
		mu.Unlock()
		if err != nil {
			errCount.Add(1)
		} else {
			okCount.Add(1)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg.StartPeriodicRevalidation(ctx, 50*time.Millisecond, observer)

	// Wait for the ticker to fire at least once after tampering.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	// Give the goroutine a moment to exit cleanly.
	time.Sleep(50 * time.Millisecond)

	if errCount.Load() == 0 {
		t.Fatalf("expected periodic revalidation to detect tampering (errCount=0, okCount=%d)", okCount.Load())
	}
	mu.Lock()
	gotErr := lastErr
	mu.Unlock()
	if gotErr == nil {
		t.Fatal("expected non-nil error from observer on tampered profile")
	}
}

// TestProfilePeriodicRevalidation_HealthyBeforeTamper verifies the ticker
// reports OK while the file is unmodified.
func TestProfilePeriodicRevalidation_HealthyBeforeTamper(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	original := []byte("profile: test\nsecurity_tier: dev\n")
	os.WriteFile(profilePath, original, 0644)
	sig := ed25519.Sign(priv, original)
	os.WriteFile(profilePath+".sig", sig, 0644)
	t.Setenv("OF_PROFILE_PUBKEY", hex.EncodeToString(pub))

	cfg, err := Load(profilePath, true)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var okCount atomic.Int32
	observer := func(path string, err error) {
		if err == nil {
			okCount.Add(1)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg.StartPeriodicRevalidation(ctx, 50*time.Millisecond, observer)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if okCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	time.Sleep(50 * time.Millisecond)

	if okCount.Load() == 0 {
		t.Fatal("expected at least one OK revalidation tick")
	}
}

// TestProfilePeriodicRevalidation_StopsOnContextCancel ensures the goroutine
// exits when the context is cancelled (no goroutine leak).
func TestProfilePeriodicRevalidation_StopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	os.WriteFile(profilePath, []byte("profile: test\nsecurity_tier: dev\n"), 0644)
	cfg, err := Load(profilePath, false)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg.StartPeriodicRevalidation(ctx, 10*time.Millisecond, func(string, error) {})

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}
