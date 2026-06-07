package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"openforge/internal/shared/kernel"
)

type StdoutTelemetry struct{}

func New() *StdoutTelemetry { return &StdoutTelemetry{} }

func (t *StdoutTelemetry) Trace(ctx context.Context, name string) (context.Context, kernel.Span) {
	return ctx, &stdoutSpan{name: name, start: time.Now()}
}

func (t *StdoutTelemetry) Log(level string, msg string, fields map[string]any) {
	if ShouldSample(parseLevel(level), msg) == Drop {
		return
	}
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	b, _ := json.Marshal(entry)
	fmt.Fprintln(os.Stderr, string(b))
}

func (t *StdoutTelemetry) Metric(name string, value float64, tags map[string]string) {}

// parseLevel maps the human-readable level string passed to Log() to the
// matching slog.Level so that ShouldSample can apply the right policy.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Unknown / empty -> treat as INFO (the safe default that goes through
		// the 10% sampling path).
		return slog.LevelInfo
	}
}

type stdoutSpan struct {
	name  string
	start time.Time
}

func (s *stdoutSpan) End()                                       {}
func (s *stdoutSpan) AddEvent(name string, attrs map[string]string) {}
