package adapter

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"openforge/internal/observability/domain"
)

// PrometheusExporter serves OpenForge metrics in Prometheus text format on a
// dedicated HTTP endpoint (default ":9090").  It is designed to run in a
// separate goroutine from the main application server.
//
// The exporter registers the standard counters and gauges defined in
// internal/observability/domain/metrics.go.  Call IncrementCounter / SetGauge
// from telemetry hooks throughout the application.
type PrometheusExporter struct {
	counters map[string]*atomic.Int64
	gauges   map[string]*atomic.Int64
	mu       sync.RWMutex // protects maps only; individual counters/gauges are atomic

	server *http.Server
}

// NewPrometheusExporter creates an exporter with the standard OpenForge
// counters and gauges pre-registered:
//
//	Counters: of_pipeline_created_total, of_agent_llm_call_errors_total
//	Gauges:   of_circuit_breaker_state
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{
		counters: map[string]*atomic.Int64{
			string(domain.MetricPipelineCreated):   new(atomic.Int64),
			string(domain.MetricLLMCallErrors):     new(atomic.Int64),
		},
		gauges: map[string]*atomic.Int64{
			string(domain.MetricCircuitBreaker): new(atomic.Int64),
		},
	}
}

// Listen starts the Prometheus HTTP server on addr (e.g. ":9090").  It blocks
// until the server fails; call it inside a goroutine.
func (pe *PrometheusExporter) Listen(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", pe.handleMetrics)
	pe.server = &http.Server{Addr: addr, Handler: mux}

	log.Printf("[prometheus] metrics exporter listening on %s/metrics", addr)
	return pe.server.ListenAndServe()
}

// IncrementCounter adds delta to the named counter.  No-op if the name was
// not registered via NewPrometheusExporter.
func (pe *PrometheusExporter) IncrementCounter(name string, delta int64) {
	pe.mu.RLock()
	c, ok := pe.counters[name]
	pe.mu.RUnlock()
	if ok {
		c.Add(delta)
	}
}

// SetGauge sets a named gauge to value.  No-op if the name was not registered
// via NewPrometheusExporter.
func (pe *PrometheusExporter) SetGauge(name string, value int64) {
	pe.mu.RLock()
	g, ok := pe.gauges[name]
	pe.mu.RUnlock()
	if ok {
		g.Store(value)
	}
}

// Snapshot returns a copy of all current metric values.  Used by tests and
// for integration with the admin status endpoint.
func (pe *PrometheusExporter) Snapshot() (counters map[string]int64, gauges map[string]int64) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	counters = make(map[string]int64, len(pe.counters))
	for name, c := range pe.counters {
		counters[name] = c.Load()
	}

	gauges = make(map[string]int64, len(pe.gauges))
	for name, g := range pe.gauges {
		gauges[name] = g.Load()
	}
	return
}

// FormatMetrics returns the /metrics response body as a string in Prometheus
// text format (content-type text/plain; version=0.0.4).  Exported for tests.
func (pe *PrometheusExporter) FormatMetrics() string {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var out string
	for name, c := range pe.counters {
		out += fmt.Sprintf("# HELP %s OpenForge metric\n", name)
		out += fmt.Sprintf("# TYPE %s counter\n", name)
		out += fmt.Sprintf("%s %d\n", name, c.Load())
	}
	for name, g := range pe.gauges {
		out += fmt.Sprintf("# HELP %s OpenForge metric\n", name)
		out += fmt.Sprintf("# TYPE %s gauge\n", name)
		out += fmt.Sprintf("%s %d\n", name, g.Load())
	}
	return out
}

// handleMetrics serves the /metrics endpoint in Prometheus text format.
func (pe *PrometheusExporter) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprint(w, pe.FormatMetrics())
}

// Close shuts down the metrics HTTP server.  Safe to call even if Listen was
// never called.
func (pe *PrometheusExporter) Close() error {
	if pe.server != nil {
		return pe.server.Close()
	}
	return nil
}
