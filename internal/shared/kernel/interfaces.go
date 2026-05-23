package kernel

import (
	"context"
	"io"
	"time"
)

// Message is a unit of work sent through the task queue.
type Message struct {
	ID       string
	Payload  []byte
	Priority int
}

// Event is a notification published through the event bus.
type Event struct {
	Type    string
	Payload []byte
}

// Target describes where a notification should be delivered.
type Target struct {
	Email    string
	Webhook  string
	Channels []string
}

// Notification is the payload delivered to a target.
type Notification struct {
	Level     string
	Title     string
	Body      string
	ActionURL string
}

// ContainerSpec describes how to create a sandbox container.
type ContainerSpec struct {
	Image   string
	Workdir string
	Env     []string
	Cmd     []string
}

// Container represents a running sandbox container.
type Container struct {
	ID     string
	Status string
}

// Span represents a unit of work in a distributed trace.
type Span interface {
	End()
	AddEvent(name string, attrs map[string]string)
}

// DRStatus reports the health of disaster recovery.
type DRStatus struct {
	Healthy     bool
	LastBackup  time.Time
	LastRestore time.Time
}

// TaskQueue is a point-to-point FIFO queue with priority.
type TaskQueue interface {
	Enqueue(ctx context.Context, topic string, msg Message, priority int) error
	Dequeue(ctx context.Context, topic string) (Message, error)
	Ack(ctx context.Context, topic string, msgID string) error
}

// EventBus is a publish-subscribe broadcast bus.
type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	Subscribe(ctx context.Context, topic string) (<-chan Event, error)
}

// Notifier sends notifications with retry and dead-letter.
type Notifier interface {
	Send(ctx context.Context, target Target, msg Notification) error
	SendWithRetry(ctx context.Context, target Target, msg Notification, maxRetries int) error
}

// ContainerRuntime manages sandbox containers.
type ContainerRuntime interface {
	Create(ctx context.Context, spec ContainerSpec) (Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	List(ctx context.Context) ([]Container, error)
}

// SecretStore reads secrets.
type SecretStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// ObjectStore is blob/artifact storage.
type ObjectStore interface {
	Put(ctx context.Context, key string, reader io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// Cache is a key-value store with TTL.
type Cache interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// Telemetry provides tracing, metrics, and logging.
type Telemetry interface {
	Trace(ctx context.Context, name string) (context.Context, Span)
	Log(level string, msg string, fields map[string]any)
	Metric(name string, value float64, tags map[string]string)
}

// ServiceRegistry discovers services by name.
type ServiceRegistry interface {
	Register(ctx context.Context, name string, addr string) error
	Discover(ctx context.Context, name string) ([]string, error)
	Watch(ctx context.Context, name string) (<-chan Event, error)
}

// DisasterRecovery handles backup and restore.
type DisasterRecovery interface {
	Backup(ctx context.Context) error
	Restore(ctx context.Context, point time.Time) error
	Status(ctx context.Context) (DRStatus, error)
}

// LoadBalancer manages backend pools.
type LoadBalancer interface {
	AddBackend(ctx context.Context, name string, addr string) error
	RemoveBackend(ctx context.Context, name string, addr string) error
	HealthCheck(ctx context.Context, name string) (bool, error)
}

// --- CommandExecutor (12th capability domain) ---

// ExecOptions configures command execution.
type ExecOptions struct {
	WorkDir   string
	Env       map[string]string
	Timeout   time.Duration
	MaxOutput int64
	ReadOnly  bool
}

// ExecOutput holds the result of a command execution.
type ExecOutput struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// ExecStreamChunk represents a single chunk of streaming command output.
type ExecStreamChunk struct {
	Delta  string
	Stream string // "stdout" | "stderr"
}

// CommandExecutor executes shell commands. Implementation is selected by
// Profile: local-shell (minimal, zero-dependency) or docker-sandbox (standard+).
type CommandExecutor interface {
	Execute(ctx context.Context, command string, opts ExecOptions) (ExecOutput, error)
	ExecuteStream(ctx context.Context, command string, opts ExecOptions) (<-chan ExecStreamChunk, error)
	Validate(ctx context.Context, command string, opts ExecOptions) error
}
