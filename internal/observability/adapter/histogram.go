package adapter

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// Histogram is a minimal in-memory histogram suitable for emitting a
// Prometheus count + sum pair from FormatMetrics.  Bucket boundaries are
// expressed in seconds; observations are recorded as float64 values.
//
// This is intentionally simple: no quantile sketches, no exemplar storage.
// The exporter surfaces _count and _sum for each histogram and a fixed
// bucket layout.  When production observability requires real
// histograms, swap this implementation for prometheus.HistogramVec.
type Histogram struct {
	mu      sync.RWMutex
	count   uint64
	sumBits uint64 // float64 stored as bits via math.Float64bits
	buckets []float64
	counts  []uint64
}

// DefaultBuckets is a reasonable default for second-scale latencies.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// NewHistogram creates a histogram with the default bucket layout.
func NewHistogram() *Histogram {
	return NewHistogramWithBuckets(DefaultBuckets)
}

// NewHistogramWithBuckets creates a histogram with a custom bucket layout.
// buckets must be sorted ascending.
func NewHistogramWithBuckets(buckets []float64) *Histogram {
	cp := make([]float64, len(buckets))
	copy(cp, buckets)
	sort.Float64s(cp)
	return &Histogram{
		buckets: cp,
		counts:  make([]uint64, len(cp)),
	}
}

// Observe records a single observation.
func (h *Histogram) Observe(value float64) {
	atomic.AddUint64(&h.count, 1)
	for i, b := range h.buckets {
		if value <= b {
			atomic.AddUint64(&h.counts[i], 1)
		}
	}
	// Atomic add to the running sum.
	for {
		old := atomic.LoadUint64(&h.sumBits)
		oldF := math.Float64frombits(old)
		newF := oldF + value
		newBits := math.Float64bits(newF)
		if atomic.CompareAndSwapUint64(&h.sumBits, old, newBits) {
			return
		}
	}
}

// Count returns the number of observations.
func (h *Histogram) Count() uint64 { return atomic.LoadUint64(&h.count) }

// Sum returns the total of all observations.
func (h *Histogram) Sum() float64 { return math.Float64frombits(atomic.LoadUint64(&h.sumBits)) }

// Snapshot returns the bucket counts and bucket boundaries.  Used by
// FormatMetrics to render Prometheus text output.
func (h *Histogram) Snapshot() (buckets []float64, counts []uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	buckets = make([]float64, len(h.buckets))
	counts = make([]uint64, len(h.counts))
	copy(buckets, h.buckets)
	for i := range h.counts {
		counts[i] = atomic.LoadUint64(&h.counts[i])
	}
	return
}

// Format renders the histogram in Prometheus text format:
//
//	# HELP name OpenForge metric
//	# TYPE name histogram
//	name_bucket{le="0.005"} 0
//	name_bucket{le="+Inf"} 0
//	name_count 0
//	name_sum 0
func (h *Histogram) Format(name string) string {
	buckets, counts := h.Snapshot()
	out := fmt.Sprintf("# HELP %s OpenForge metric\n", name)
	out += fmt.Sprintf("# TYPE %s histogram\n", name)
	for i, b := range buckets {
		out += fmt.Sprintf("%s_bucket{le=\"%g\"} %d\n", name, b, counts[i])
	}
	out += fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", name, h.Count())
	out += fmt.Sprintf("%s_count %d\n", name, h.Count())
	out += fmt.Sprintf("%s_sum %g\n", name, h.Sum())
	return out
}
