package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"openforge/internal/shared/kernel"
)

type StdoutTelemetry struct{}

func New() *StdoutTelemetry { return &StdoutTelemetry{} }

func (t *StdoutTelemetry) Trace(ctx context.Context, name string) (context.Context, kernel.Span) {
	return ctx, &stdoutSpan{name: name, start: time.Now()}
}

func (t *StdoutTelemetry) Log(level string, msg string, fields map[string]any) {
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

type stdoutSpan struct {
	name  string
	start time.Time
}

func (s *stdoutSpan) End()                                       {}
func (s *stdoutSpan) AddEvent(name string, attrs map[string]string) {}
