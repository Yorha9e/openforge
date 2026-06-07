// 4.3 -- Sustained high latency.
//
// Validates that an application can tolerate a moderate (200 ms) per-request
// latency from a degraded dependency.  A 200 ms tax is well within most
// HTTP/gRPC deadline budgets (commonly 1-5s) so the client should *succeed*,
// just take a measurable amount of wall-clock time.
package chaos

import (
	"testing"
	"time"
)

func TestChaos_HighLatency_ApplicationTolerates(t *testing.T) {
	if testing.Short() {
		t.Skip("requires toxiproxy container; run without -short")
	}

	// Reuse the in-process echo helper from slow_redis_test.go.
	ln, err := netListenLoopback(t)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	tox := NewToxiproxyHelper(t, "http://localhost:8474")
	tox.AddTCPProxy(t, "latency-tolerate", "0.0.0.0:26380", ln.Addr().String())

	// 1. Baseline.
	baseline := roundTrip(t, "localhost:26380", 500*time.Millisecond)
	t.Logf("baseline roundtrip: %v", baseline)

	// 2. Inject 200 ms latency.
	tox.AddLatency(t, "latency-tolerate", 200*time.Millisecond)

	// 3. Round-trip must succeed and take >= 200 ms.
	const allowedMax = 1 * time.Second
	start := time.Now()
	took := roundTrip(t, "localhost:26380", allowedMax)
	if took < 200*time.Millisecond {
		t.Fatalf("expected >= 200ms with latency toxic, got %v", took)
	}
	if took > allowedMax {
		t.Fatalf("roundtrip took %v, exceeds tolerance %v", took, allowedMax)
	}
	t.Logf("latency-toxic roundtrip: %v (budget %v, wall %v)", took, allowedMax, time.Since(start))
}
