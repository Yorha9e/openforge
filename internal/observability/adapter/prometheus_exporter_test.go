package adapter

import (
	"strings"
	"testing"

	"openforge/internal/observability/domain"
)

func TestNewPrometheusExporter(t *testing.T) {
	pe := NewPrometheusExporter()
	if pe == nil {
		t.Fatal("expected non-nil exporter")
	}

	counters, gauges := pe.Snapshot()
	if want := 2; len(counters) != want {
		t.Errorf("expected %d counters, got %d", want, len(counters))
	}
	if want := 1; len(gauges) != want {
		t.Errorf("expected %d gauge, got %d", want, len(gauges))
	}

	// Verify specific metric names.
	if _, ok := counters[string(domain.MetricPipelineCreated)]; !ok {
		t.Errorf("missing counter %s", domain.MetricPipelineCreated)
	}
	if _, ok := counters[string(domain.MetricLLMCallErrors)]; !ok {
		t.Errorf("missing counter %s", domain.MetricLLMCallErrors)
	}
	if _, ok := gauges[string(domain.MetricCircuitBreaker)]; !ok {
		t.Errorf("missing gauge %s", domain.MetricCircuitBreaker)
	}
}

func TestIncrementCounter(t *testing.T) {
	pe := NewPrometheusExporter()

	pe.IncrementCounter(string(domain.MetricPipelineCreated), 5)
	pe.IncrementCounter(string(domain.MetricLLMCallErrors), 2)

	counters, _ := pe.Snapshot()
	if got := counters[string(domain.MetricPipelineCreated)]; got != 5 {
		t.Errorf("pipeline_created = %d, want 5", got)
	}
	if got := counters[string(domain.MetricLLMCallErrors)]; got != 2 {
		t.Errorf("llm_call_errors = %d, want 2", got)
	}

	// Increment existing counter further.
	pe.IncrementCounter(string(domain.MetricPipelineCreated), 1)
	counters, _ = pe.Snapshot()
	if got := counters[string(domain.MetricPipelineCreated)]; got != 6 {
		t.Errorf("pipeline_created after second increment = %d, want 6", got)
	}
}

func TestSetGauge(t *testing.T) {
	pe := NewPrometheusExporter()

	pe.SetGauge(string(domain.MetricCircuitBreaker), 1) // CLOSED
	_, gauges := pe.Snapshot()
	if got := gauges[string(domain.MetricCircuitBreaker)]; got != 1 {
		t.Errorf("circuit_breaker = %d, want 1", got)
	}

	pe.SetGauge(string(domain.MetricCircuitBreaker), 0) // OPEN
	_, gauges = pe.Snapshot()
	if got := gauges[string(domain.MetricCircuitBreaker)]; got != 0 {
		t.Errorf("circuit_breaker = %d, want 0", got)
	}

	pe.SetGauge(string(domain.MetricCircuitBreaker), 2) // HALF_OPEN
	_, gauges = pe.Snapshot()
	if got := gauges[string(domain.MetricCircuitBreaker)]; got != 2 {
		t.Errorf("circuit_breaker = %d, want 2", got)
	}
}

func TestIncrementUnknownCounter(t *testing.T) {
	pe := NewPrometheusExporter()
	// Should not panic — just a no-op.
	pe.IncrementCounter("of_unknown_metric", 1)
}

func TestSetUnknownGauge(t *testing.T) {
	pe := NewPrometheusExporter()
	// Should not panic — just a no-op.
	pe.SetGauge("of_unknown_gauge", 42)
}

func TestFormatMetrics(t *testing.T) {
	pe := NewPrometheusExporter()
	pe.IncrementCounter(string(domain.MetricPipelineCreated), 3)
	pe.SetGauge(string(domain.MetricCircuitBreaker), 1)

	output := pe.FormatMetrics()

	// HELP lines.
	if !strings.Contains(output, "# HELP of_pipeline_created_total") {
		t.Error("missing HELP for of_pipeline_created_total")
	}
	if !strings.Contains(output, "# HELP of_circuit_breaker_state") {
		t.Error("missing HELP for of_circuit_breaker_state")
	}

	// TYPE lines.
	if !strings.Contains(output, "# TYPE of_pipeline_created_total counter") {
		t.Error("missing TYPE counter for of_pipeline_created_total")
	}
	if !strings.Contains(output, "# TYPE of_circuit_breaker_state gauge") {
		t.Error("missing TYPE gauge for of_circuit_breaker_state")
	}

	// Values.
	if !strings.Contains(output, "of_pipeline_created_total 3") {
		t.Error("expected pipeline_created_total=3 in output")
	}
	if !strings.Contains(output, "of_circuit_breaker_state 1") {
		t.Error("expected circuit_breaker_state=1 in output")
	}

	// Zero-value metrics should also appear (llm_call_errors was never incremented).
	if !strings.Contains(output, "of_agent_llm_call_errors_total 0") {
		t.Error("expected llm_call_errors=0 in output")
	}

	// Verify correct type annotation (no counter/gauge confusion).
	if strings.Contains(output, "# TYPE of_circuit_breaker_state counter") {
		t.Error("circuit_breaker_state should be gauge, not counter")
	}
	if strings.Contains(output, "# TYPE of_pipeline_created_total gauge") {
		t.Error("pipeline_created_total should be counter, not gauge")
	}
}

func TestFormatMetricsOrder(t *testing.T) {
	pe := NewPrometheusExporter()
	output := pe.FormatMetrics()

	// Prometheus text format does not mandate ordering, but each metric must
	// have its HELP immediately followed by its TYPE line.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "# HELP ") {
			if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "# TYPE ") {
				t.Errorf("HELP line at position %d not followed by TYPE: %s", i, lines[i])
			}
			i++ // skip the TYPE line we just validated
		}
	}
}

func TestSnapshotConsistency(t *testing.T) {
	pe := NewPrometheusExporter()

	pe.IncrementCounter(string(domain.MetricPipelineCreated), 10)

	counters1, gauges1 := pe.Snapshot()
	counters2, gauges2 := pe.Snapshot()

	// Each call must return a fresh map — mutating one must not affect the other.
	counters2[string(domain.MetricPipelineCreated)] = 999
	if counters1[string(domain.MetricPipelineCreated)] == 999 {
		t.Error("Snapshot returned aliased map; mutating copy affected original")
	}

	gauges2[string(domain.MetricCircuitBreaker)] = 999
	if gauges1[string(domain.MetricCircuitBreaker)] == 999 {
		t.Error("Snapshot returned aliased map; mutating copy affected original")
	}
}

func TestClose(t *testing.T) {
	pe := NewPrometheusExporter()
	if err := pe.Close(); err != nil {
		t.Errorf("Close on unstarted server: %v", err)
	}

	// Double-close should also be safe.
	if err := pe.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
