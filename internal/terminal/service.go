package terminal

import (
	"context"
	"log/slog"
	"sync"

	"openforge/internal/shared/kernel"
)

// Service is the in-pipeline sandbox terminal adapter used by the WS
// layer. Path C T4 ships a minimal implementation that fans out input
// to all registered terminal streams. Future work: bind to the actual
// ContainerRuntime and stream the underlying PTY.
type Service struct {
	mu      sync.Mutex
	streams map[string]chan string // pipelineID → input channel
	exec    kernel.CommandExecutor
}

// NewService builds a Service backed by the given command executor. The
// executor may be nil for tests or for profiles that don't enable a
// sandbox runtime — in that case Input logs and returns nil.
func NewService(exec kernel.CommandExecutor) *Service {
	return &Service{
		streams: make(map[string]chan string),
		exec:    exec,
	}
}

// Input forwards the given string to the terminal stream registered for
// the pipeline. If no stream is registered (e.g. the sandbox hasn't been
// started for this pipeline yet) the call is logged and treated as a
// no-op — clients are expected to start the stream before sending input.
func (s *Service) Input(ctx context.Context, pipelineID, input string) error {
	if pipelineID == "" {
		return nil
	}
	s.mu.Lock()
	ch, ok := s.streams[pipelineID]
	s.mu.Unlock()
	if !ok {
		slog.Debug("terminal input dropped: no active stream",
			"pipeline_id", pipelineID,
			"input_len", len(input),
		)
		return nil
	}
	select {
	case ch <- input:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
