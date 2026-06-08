package bench

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"openforge/internal/agent/service"
)

// BenchmarkCSPChannel_Latency measures round-trip latency of
// service.CSPChannel (the buffered channel + WAL backing store used by the
// agent message bus). Reports p50/p95/p99 microseconds in addition to the
// default ns/op.
//
// Always runnable: the channel is in-process and has no external deps.
func BenchmarkCSPChannel_Latency(b *testing.B) {
	// Buffer must be > b.N to keep the hot path on the channel (not the
	// WAL). 1<<20 covers default 2s benchtime on commodity hardware; for
	// longer runs the bench will still degrade gracefully because the
	// bench file records the actual Send -> Receive latency even when
	// the WAL is touched.
	const bufferSize = 1 << 20
	ch := service.NewCSPChannel("bench", bufferSize)
	defer ch.Drain(context.Background())

	// Latency samples indexed by send sequence number; receiver records
	// the elapsed time from Send -> Receive.
	latencies := make([]time.Duration, b.N)
	starts := make([]time.Time, b.N)
	seqCh := make(chan int, b.N)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		recv := ch.Receive()
		for i := 0; i < b.N; i++ {
			msg, ok := <-recv
			if !ok {
				return
			}
			// Body carries the send-side sequence number so the receiver
			// can index into starts/latencies without an extra atomic.
			idx := int(msg.Body[0])
			latencies[idx] = time.Since(starts[idx])
		}
		close(seqCh)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		starts[i] = time.Now()
		// Body[0] = sequence number; cheap wire format.
		if err := ch.Send(context.Background(), service.Message{Body: []byte{byte(i)}}); err != nil {
			b.Fatalf("send %d: %v", i, err)
		}
	}
	wg.Wait()

	// Sort once and report percentiles. For very small b.N the
	// percentile indices collapse to b.N-1, which is the max sample —
	// semantically still "the slowest of N" and a valid latency ceiling.
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	pick := func(pct int) int {
		idx := b.N * pct / 100
		if idx >= b.N {
			idx = b.N - 1
		}
		return idx
	}
	b.ReportMetric(float64(latencies[pick(50)].Microseconds()), "p50-us")
	b.ReportMetric(float64(latencies[pick(95)].Microseconds()), "p95-us")
	b.ReportMetric(float64(latencies[pick(99)].Microseconds()), "p99-us")
}
