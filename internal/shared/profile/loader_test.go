package profile

import (
	"os"
	"path/filepath"
	"testing"
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
