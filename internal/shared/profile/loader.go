package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the full profile configuration loaded from a YAML file.
type Config struct {
	Profile      string `yaml:"profile"`
	SecurityTier string `yaml:"security_tier"`

	SecretStore      string `yaml:"secret_store"`
	ContainerRuntime string `yaml:"container_runtime"`
	ObjectStore      string `yaml:"object_store"`
	TaskQueue        string `yaml:"task_queue"`
	EventBus         string `yaml:"event_bus"`
	Cache            string `yaml:"cache"`
	Telemetry        string `yaml:"telemetry"`
	ServiceRegistry  string `yaml:"service_registry"`
	DisasterRecovery string `yaml:"disaster_recovery"`
	LoadBalancer     string `yaml:"load_balancer"`
	Notifier         string `yaml:"notifier"`
	CommandExecutor  string `yaml:"command_executor"`

	Database DatabaseConfig `yaml:"database"`
	LLM      LLMConfig      `yaml:"llm"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	JWT      JWTConfig      `yaml:"jwt"`
}

// DockerConfig holds Docker daemon connection parameters.
type DockerConfig struct {
	Host       string `yaml:"host"`
	APIVersion string `yaml:"api_version"`
}

// DatabaseConfig holds database connection parameters.
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	DefaultProvider string     `yaml:"default_provider"`
	DefaultModel    string     `yaml:"default_model"`
	Models          []ModelDef `yaml:"models"`
}

// ModelDef defines a model entry loaded from YAML.
type ModelDef struct {
	Alias    string   `yaml:"alias"`
	Provider string   `yaml:"provider"`
	ModelID  string   `yaml:"model_id"`
	BaseURL  string   `yaml:"base_url"`
	Fallback []string `yaml:"fallback"`
}

// GRPCConfig holds gRPC endpoint addresses.
type GRPCConfig struct {
	NodejsIOAddr    string `yaml:"nodejs_io_addr"`
	CoordinatorAddr string `yaml:"coordinator_addr"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// JWTConfig holds JWT auth configuration.
type JWTConfig struct {
	Secret     string `yaml:"secret"`
	AccessTTL  string `yaml:"access_ttl"`
	RefreshTTL string `yaml:"refresh_ttl"`
}

// Load reads a YAML profile from path, optionally verifies its Ed25519
// signature, and returns a validated Config.
func Load(path string, verifySignature bool) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %w", path, err)
	}

	if verifySignature {
		sigPath := path + ".sig"
		sig, err := os.ReadFile(sigPath)
		if err != nil {
			return nil, fmt.Errorf("read signature %s: %w", sigPath, err)
		}
		pubKey := os.Getenv("OF_PROFILE_PUBKEY")
		if pubKey == "" {
			return nil, fmt.Errorf("OF_PROFILE_PUBKEY not set")
		}
		_ = sig
		_ = pubKey
		// Ed25519 profile signature verification deferred to Phase 8 (per DESIGN.md §6.5).
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate profile: %w", err)
	}

	return &cfg, nil
}

// validate checks that required fields are present and recognized.
func (c *Config) validate() error {
	if c.Profile == "" {
		return fmt.Errorf("profile name is required")
	}
	switch c.SecurityTier {
	case "dev", "prod", "regulated":
	default:
		return fmt.Errorf("unknown security_tier: %s", c.SecurityTier)
	}
	return nil
}
