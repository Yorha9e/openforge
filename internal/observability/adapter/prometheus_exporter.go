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
	histograms map[string]*Histogram
	mu       sync.RWMutex // protects maps only; individual counters/gauges are atomic

	server *http.Server
}

// NewPrometheusExporter creates an exporter with the standard OpenForge
// counters and gauges pre-registered:
//
//	Counters: 8 (pipeline created/completed, llm call errors, token usage,
//	          backtrack, gate approve/reject, learning fallback)
//	Gauges:   5 (code acceptance rate, goroutine count, circuit breaker state,
//	          sandbox pool size, token quota remaining)
//	Histograms: 2 (pipeline duration, llm call duration)
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{
		counters: map[string]*atomic.Int64{
			string(domain.MetricPipelineCreated):    new(atomic.Int64),
			string(domain.MetricPipelineCompleted):  new(atomic.Int64),
			string(domain.MetricLLMCallErrors):      new(atomic.Int64),
			string(domain.MetricTokenUsage):         new(atomic.Int64),
			string(domain.MetricBacktrackTotal):     new(atomic.Int64),
			string(domain.MetricGateApproveTotal):   new(atomic.Int64),
			string(domain.MetricGateRejectTotal):    new(atomic.Int64),
			string(domain.MetricLearningFallback):   new(atomic.Int64),
		},
		gauges: map[string]*atomic.Int64{
			string(domain.MetricCodeAcceptanceRate): new(atomic.Int64),
			string(domain.MetricGoroutineCount):     new(atomic.Int64),
			string(domain.MetricCircuitBreaker):     new(atomic.Int64),
			string(domain.MetricTokenQuotaRemaining): new(atomic.Int64),
			string(domain.MetricSandboxPool):         new(atomic.Int64),
		},
		histograms: map[string]*Histogram{
			string(domain.MetricPipelineDuration): NewHistogram(),
			string(domain.MetricLLMCallDuration):  NewHistogram(),
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

// Incr is a convenience helper that increments a counter by 1.  Maps to
// IncrementCounter(name, 1).  No-op if the name is not registered.  This is
// the preferred call-site spelling for telemetry hooks.
func (pe *PrometheusExporter) Incr(name string) {
	pe.IncrementCounter(name, 1)
}

// Set is a convenience helper that sets a gauge to an integer value.  Maps
// to SetGauge(name, int64(value)).  No-op if the name is not registered.
func (pe *PrometheusExporter) Set(name string, value int64) {
	pe.SetGauge(name, value)
}

// Observe records a single observation on a named histogram.  No-op if the
// histogram is not registered.  Histogram observations are tracked
// in-memory and rendered in the /metrics output as a count + sum pair.
func (pe *PrometheusExporter) Observe(name string, value float64) {
	pe.mu.RLock()
	h, ok := pe.histograms[name]
	pe.mu.RUnlock()
	if ok {
		h.Observe(value)
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
	for name, h := range pe.histograms {
		out += h.Format(name)
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
