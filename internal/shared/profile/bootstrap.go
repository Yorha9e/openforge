package profile

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"openforge/internal/adapter"
	"openforge/internal/agent/domain"
	"openforge/internal/llm"
	pipelineadapter "openforge/internal/pipeline/adapter"
	"openforge/internal/pipeline/service"
	"openforge/internal/shared/kernel"
)

// OpenForge is the composition root of all 10 capability domains. It is
// constructed by Bootstrap and provides ready-to-use implementations of
// every interface declared in kernel/interfaces.go.
type OpenForge struct {
	Secrets      kernel.SecretStore
	Container    kernel.ContainerRuntime
	Object       kernel.ObjectStore
	TaskQ        kernel.TaskQueue
	Events       kernel.EventBus
	Cache        kernel.Cache
	Telemetry    kernel.Telemetry
	Registry     kernel.ServiceRegistry
	DR           kernel.DisasterRecovery
	LB           kernel.LoadBalancer
	Notifier     kernel.Notifier
	CommandExec  kernel.CommandExecutor
	LLMRouter    *llm.Router
	LLMRegistry     *llm.Registry
	Config          *Config
	PromptBuilder   *domain.PromptBuilder
	SkillLoader         *domain.SkillLoader
	CapabilityInjector  *domain.CapabilityInjector
	PriorityEngine      *domain.UnifiedPriorityEngine

	PipelineRepo    *pipelineadapter.PGRepository
	PipelineSvc     *service.PipelineService
	GateSvc         *service.GateService
	SandboxProvider *adapter.SandboxProvider
	DeploySvc       *service.DeployService
	TokenCostSvc    *service.TokenCostService
	DB              *sql.DB
}

// Bootstrap creates a new OpenForge composition root from the given profile
// configuration. It enforces the invariant that "minimal" profile may not be
// used in prod or regulated security tiers, then instantiates concrete
// implementations for all 11 kernel interfaces.
func Bootstrap(cfg *Config) (*OpenForge, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Safety invariant: minimal profile prohibited in production tiers.
	if cfg.SecurityTier == "prod" || cfg.SecurityTier == "regulated" {
		if cfg.Profile == "minimal" {
			return nil, fmt.Errorf("FATAL: minimal profile prohibited in %s tier", cfg.SecurityTier)
		}
	}

	of := &OpenForge{Config: cfg}
	of.Secrets = newSecretStore(cfg)
	of.Container = newContainerRuntime(cfg)
	of.Object = newObjectStore(cfg)
	of.TaskQ = newTaskQueue(cfg)
	of.Events = newEventBus(cfg)
	of.Cache = newCache(cfg)
	of.Telemetry = newTelemetry(cfg)
	of.Registry = newServiceRegistry(cfg)
	of.DR = newDisasterRecovery(cfg)
	of.LB = newLoadBalancer(cfg)
	of.Notifier = newNotifier(cfg)
	of.CommandExec = newCommandExecutor(cfg)
	llmRegistry := llm.NewRegistry()
	of.LLMRegistry = llmRegistry

	// Load model definitions from profile YAML (overrides hardcoded seeds).
	if len(cfg.LLM.Models) > 0 {
		modelDefs := make([]llm.ModelDef, len(cfg.LLM.Models))
		for i, m := range cfg.LLM.Models {
			modelDefs[i] = llm.ModelDef{
				Alias:    m.Alias,
				Provider: m.Provider,
				ModelID:  m.ModelID,
				BaseURL:  m.BaseURL,
				Fallback: m.Fallback,
			}
		}
		llmRegistry.LoadFromConfig(modelDefs)
	}

	of.LLMRouter = llm.NewRouter(llmRegistry, of.Secrets)

	// Register LLM providers (keys resolved at bootstrap time).
	antAPIKey, _ := of.Secrets.Get(context.Background(), "ANTHROPIC_AUTH_TOKEN")
	dsAPIKey, errDS := of.Secrets.Get(context.Background(), "DEEPSEEK_AUTH_TOKEN")
	if errDS != nil {
		dsAPIKey = antAPIKey // fallback: DeepSeek reuses Anthropic key
	}
	of.LLMRouter.RegisterProvider("anthropic", llm.NewAnthropicProvider(
		"https://api.anthropic.com", string(antAPIKey)))
	of.LLMRouter.RegisterProvider("deepseek", llm.NewDeepSeekProvider(
		"https://api.deepseek.com/anthropic", string(dsAPIKey)))
	openAIKey, errOAI := of.Secrets.Get(context.Background(), "OPENAI_API_KEY")
	if errOAI != nil {
		openAIKey = antAPIKey // fallback for dev: reuse Anthropic key
	}
	of.LLMRouter.RegisterProvider("openai", llm.NewOpenAIProvider(
		"https://api.openai.com", string(openAIKey)))

	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	of.DB = db
	of.PipelineRepo = pipelineadapter.NewPGRepository(db)
	of.PipelineSvc = service.NewPipelineService(of.PipelineRepo)
	of.GateSvc = service.NewGateService(of.PipelineRepo, of.PipelineRepo)

	// Phase 4: Sandbox + Deploy + Cost
	of.SandboxProvider = adapter.NewSandboxProvider(adapter.SandboxProviderConfig{
		WarmCount:   10,
		MaxTotal:    30,
		IdleTimeout: 10 * time.Minute,
		Image:       "openforge/sandbox-node:latest",
	})
	of.DeploySvc = service.NewDeployService(of.CommandExec)
	of.TokenCostSvc = service.NewTokenCostService(of.PipelineRepo)
	of.PromptBuilder, _ = domain.NewPromptBuilder("config/prompts/static.xml", nil)

	// Phase 6.5: Skill system initialization
	of.SkillLoader, _ = domain.NewSkillLoader([]string{
		"config/skills/global",
		".openforge/team-skills",
		".openforge/skills",
	})
	if of.SkillLoader != nil && of.PromptBuilder != nil {
		of.CapabilityInjector = domain.NewCapabilityInjector(of.SkillLoader, &domain.HardcodedToolRegistry{})
		of.PromptBuilder.SetCapabilityInjector(of.CapabilityInjector)
		of.PriorityEngine = domain.NewUnifiedPriorityEngine(of.SkillLoader, nil)
		of.PriorityEngine.Start()
	}

	// Run database migrations
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); err == nil {
		runner := NewMigrationRunner(db, migrationsDir)
		if err := runner.Run(context.Background()); err != nil {
			return nil, fmt.Errorf("migration: %w", err)
		}
	}
	return of, nil
}

// ---------------------------------------------------------------------------
// Minimal / stub implementations — one per kernel interface.
// ---------------------------------------------------------------------------

// --- SecretStore -----------------------------------------------------------
// ChainSecretStore tries each store in order and returns the first successful
// result. If all stores fail, the last error is returned. This enables the
// "Vault primary, env fallback" coexistence model without changing the
// SecretStore interface.

type chainSecretStore struct {
	stores []kernel.SecretStore
}

func (c *chainSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
	var lastErr error
	for _, s := range c.stores {
		val, err := s.Get(ctx, key)
		if err == nil {
			return val, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("chain: all %d stores failed for %q: %w", len(c.stores), key, lastErr)
}

type envfileSecretStore struct{}

func (s *envfileSecretStore) Get(_ context.Context, key string) ([]byte, error) {
	val := os.Getenv(key)
	if val == "" {
		return nil, fmt.Errorf("env var %q not set", key)
	}
	return []byte(val), nil
}

func newSecretStore(cfg *Config) kernel.SecretStore {
	var stores []kernel.SecretStore
	// Phase 1: envfile only. Phase 5+: prepend Vault Sidecar / Vault HA
	// implementation so it takes priority over env.
	switch cfg.SecretStore {
	case "envfile":
		stores = []kernel.SecretStore{&envfileSecretStore{}}
	case "vault-sidecar", "vault-ha":
		// Future: primary = Vault, fallback = env for local dev convenience.
		stores = []kernel.SecretStore{
			&envfileSecretStore{}, // fallback only until real Vault adapter exists
		}
	default:
		stores = []kernel.SecretStore{&envfileSecretStore{}}
	}
	if len(stores) == 1 {
		return stores[0]
	}
	return &chainSecretStore{stores: stores}
}

// --- ContainerRuntime ------------------------------------------------------

type noopContainerRuntime struct{}

func newContainerRuntime(cfg *Config) kernel.ContainerRuntime { return &noopContainerRuntime{} }

func (r *noopContainerRuntime) Create(_ context.Context, spec kernel.ContainerSpec) (kernel.Container, error) {
	return kernel.Container{}, fmt.Errorf("container runtime not available in minimal profile (image=%q)", spec.Image)
}
func (r *noopContainerRuntime) Start(_ context.Context, id string) error  { return nil }
func (r *noopContainerRuntime) Stop(_ context.Context, id string) error   { return nil }
func (r *noopContainerRuntime) Remove(_ context.Context, id string) error { return nil }
func (r *noopContainerRuntime) List(_ context.Context) ([]kernel.Container, error) {
	return nil, nil
}

// --- ObjectStore -----------------------------------------------------------

type noopObjectStore struct{}

func newObjectStore(cfg *Config) kernel.ObjectStore { return &noopObjectStore{} }

func (s *noopObjectStore) Put(_ context.Context, key string, _ io.Reader) error {
	return nil
}
func (s *noopObjectStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("object %q not found", key)
}
func (s *noopObjectStore) Delete(_ context.Context, key string) error {
	return nil
}
func (s *noopObjectStore) List(_ context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// --- TaskQueue -------------------------------------------------------------

type noopTaskQueue struct{}

func newTaskQueue(cfg *Config) kernel.TaskQueue { return &noopTaskQueue{} }

func (q *noopTaskQueue) Enqueue(_ context.Context, topic string, msg kernel.Message, priority int) error {
	return nil
}
func (q *noopTaskQueue) Dequeue(_ context.Context, topic string) (kernel.Message, error) {
	return kernel.Message{}, nil
}
func (q *noopTaskQueue) Ack(_ context.Context, topic string, msgID string) error {
	return nil
}

// --- EventBus --------------------------------------------------------------

type goroutineEventBus struct {
	mu   sync.RWMutex
	subs map[string][]chan kernel.Event
}

func newEventBus(cfg *Config) kernel.EventBus {
	return &goroutineEventBus{subs: make(map[string][]chan kernel.Event)}
}

func (b *goroutineEventBus) Publish(_ context.Context, topic string, event kernel.Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[topic] {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (b *goroutineEventBus) Subscribe(_ context.Context, topic string) (<-chan kernel.Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan kernel.Event, 64)
	b.subs[topic] = append(b.subs[topic], ch)
	return ch, nil
}

// --- Cache ----------------------------------------------------------------

type memoryCache struct {
	mu   sync.RWMutex
	data map[string]any
}

func newCache(cfg *Config) kernel.Cache { return &memoryCache{data: make(map[string]any)} }

func (c *memoryCache) Get(_ context.Context, key string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return v, nil
}
func (c *memoryCache) Set(_ context.Context, key string, val any, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = val
	return nil
}
func (c *memoryCache) Del(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

// --- Telemetry -------------------------------------------------------------

type stdoutTelemetry struct{}

func newTelemetry(cfg *Config) kernel.Telemetry { return &stdoutTelemetry{} }

func (t *stdoutTelemetry) Trace(ctx context.Context, name string) (context.Context, kernel.Span) {
	return ctx, &noopSpan{}
}
func (t *stdoutTelemetry) Log(level string, msg string, fields map[string]any) {
	fmt.Fprintf(os.Stderr, `{"level":"%s","msg":"%s","fields":%v}`+"\n", level, msg, fields)
}
func (t *stdoutTelemetry) Metric(name string, value float64, tags map[string]string) {}

type noopSpan struct{}

func (s *noopSpan) End()                                    {}
func (s *noopSpan) AddEvent(name string, attrs map[string]string) {}

// --- ServiceRegistry -------------------------------------------------------

type staticServiceRegistry struct {
	mu       sync.RWMutex
	services map[string][]string
}

func newServiceRegistry(cfg *Config) kernel.ServiceRegistry {
	services := map[string][]string{
		"llm-router":  nil,
		"coordinator": nil,
	}
	if cfg.GRPC.NodejsIOAddr != "" {
		services["llm-router"] = []string{cfg.GRPC.NodejsIOAddr}
	}
	if cfg.GRPC.CoordinatorAddr != "" {
		services["coordinator"] = []string{cfg.GRPC.CoordinatorAddr}
	}
	return &staticServiceRegistry{services: services}
}

func (r *staticServiceRegistry) Register(_ context.Context, name string, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[name] = append(r.services[name], addr)
	return nil
}
func (r *staticServiceRegistry) Discover(_ context.Context, name string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	addrs, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	copied := make([]string, len(addrs))
	copy(copied, addrs)
	return copied, nil
}
func (r *staticServiceRegistry) Watch(_ context.Context, name string) (<-chan kernel.Event, error) {
	return nil, fmt.Errorf("watch not supported in static registry")
}

// --- DisasterRecovery ------------------------------------------------------

type noopDR struct{}

func newDisasterRecovery(cfg *Config) kernel.DisasterRecovery { return &noopDR{} }

func (d *noopDR) Backup(_ context.Context) error { return nil }
func (d *noopDR) Restore(_ context.Context, _ time.Time) error {
	return nil
}
func (d *noopDR) Status(_ context.Context) (kernel.DRStatus, error) {
	return kernel.DRStatus{Healthy: true}, nil
}

// --- LoadBalancer ----------------------------------------------------------

type noopLB struct{}

func newLoadBalancer(cfg *Config) kernel.LoadBalancer { return &noopLB{} }

func (l *noopLB) AddBackend(_ context.Context, name string, addr string) error    { return nil }
func (l *noopLB) RemoveBackend(_ context.Context, name string, addr string) error { return nil }
func (l *noopLB) HealthCheck(_ context.Context, name string) (bool, error)        { return true, nil }

// --- Notifier --------------------------------------------------------------

type stdoutNotifier struct{}

func newNotifier(cfg *Config) kernel.Notifier { return &stdoutNotifier{} }

func (n *stdoutNotifier) Send(_ context.Context, target kernel.Target, msg kernel.Notification) error {
	fmt.Printf("[NOTIFY] %s | %s: %s\n", msg.Level, msg.Title, msg.Body)
	return nil
}
func (n *stdoutNotifier) SendWithRetry(_ context.Context, target kernel.Target, msg kernel.Notification, maxRetries int) error {
	return n.Send(context.Background(), target, msg)
}

func newCommandExecutor(cfg *Config) kernel.CommandExecutor {
	switch cfg.CommandExecutor {
	case "local-shell":
		return adapter.NewLocalShellExecutor(adapter.WithProfile(cfg))
	case "docker-sandbox":
		return adapter.NewLocalShellExecutor(adapter.WithProfile(cfg))
	default:
		if cfg.Profile == "minimal" {
			return adapter.NewLocalShellExecutor(adapter.WithProfile(cfg))
		}
		panic("unknown command_executor: " + cfg.CommandExecutor)
	}
}
