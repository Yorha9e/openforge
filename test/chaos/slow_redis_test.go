// 4.2 -- Slow backend (Redis-shaped).
//
// Models a degraded redis-like service by attaching a latency toxic to a
// generic TCP proxy that fronts a real netcat-style listener.  Verifies that
// the latency toxic actually delays byte transfer end-to-end.
//
// We do not pull in a redis client / miniredis here -- the test only cares
// that the *network* slows down, which is what the application has to defend
// against.  A round-trip dial + write + read is enough to detect that.
package chaos

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestChaos_SlowRedis_AppliesLatency stands up a tiny TCP echo server, fronts
// it with toxiproxy, then attaches a 1s latency toxic and asserts the
// round-trip is at least that long.
func TestChaos_SlowRedis_AppliesLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("requires toxiproxy container; run without -short")
	}

	// 1. Start an in-process TCP echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}(c)
		}
	}()

	upstream := ln.Addr().String()
	t.Logf("upstream echo server on %s", upstream)

	// 2. Create the proxy.
	tox := NewToxiproxyHelper(t, "http://localhost:8474")
	tox.AddTCPProxy(t, "redis-slow", "0.0.0.0:26379", upstream)

	// 3. Baseline ping -- should be fast (< 50 ms locally).
	baseline := roundTrip(t, "localhost:26379", 50*time.Millisecond)
	if baseline > 100*time.Millisecond {
		t.Fatalf("baseline roundtrip suspiciously slow: %v", baseline)
	}
	t.Logf("baseline roundtrip: %v", baseline)

	// 4. Inject 1000 ms latency and re-measure.
	tox.AddLatency(t, "redis-slow", 1000*time.Millisecond)

	slow := roundTrip(t, "localhost:26379", 2*time.Second)
	if slow < 900*time.Millisecond {
		t.Fatalf("expected >= 900ms after latency toxic, got %v", slow)
	}
	t.Logf("slow roundtrip: %v", slow)
}

// roundTrip dials the proxy, writes "ping", reads the echo, returns the
// elapsed time.  Fails the test if the connection can't be established.
func roundTrip(t *testing.T, addr string, deadline time.Duration) time.Duration {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, deadline)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(deadline))
	start := time.Now()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	return time.Since(start)
}
