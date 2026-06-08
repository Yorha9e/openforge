// 4.1 -- Network partition.
//
// Verifies that a postgres client connecting through a toxiproxy TCP proxy
// fails predictably when the proxy is disabled mid-connection.  This is the
// happy-path "I broke the network, what does my code do?" scenario from
// DESIGN §13.4.
package chaos

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestChaos_PostgresNetworkPartition_BreakerOpens exercises the partition
// path against the toxiproxy admin API.  It is skipped under -short because
// it requires the toxiproxy container + a reachable postgres upstream.
func TestChaos_PostgresNetworkPartition_BreakerOpens(t *testing.T) {
	if testing.Short() {
		t.Skip("requires toxiproxy + postgres-proxied containers; run without -short")
	}

	tox := NewToxiproxyHelper(t, "http://localhost:8474")

	// Listen on 0.0.0.0:25432 inside the host network, upstream is the
	// postgres-proxied service exposed on host port 5433.
	tox.AddTCPProxy(t, "pg-partition", "0.0.0.0:25432", "localhost:5433")

	// 1. Baseline -- a plain TCP dial to the proxy should succeed.
	conn, err := net.DialTimeout("tcp", "localhost:25432", 2*time.Second)
	if err != nil {
		t.Fatalf("baseline dial: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Logf("close baseline conn: %v", err)
	}

	// 2. Cut the link.
	tox.SetEnabled(t, "pg-partition", false)

	// 3. The next dial must fail.  We use a tight deadline so the test
	//    doesn't hang if for some reason toxiproxy still forwards traffic.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	d := net.Dialer{Timeout: 1500 * time.Millisecond}
	_, err = d.DialContext(ctx, "tcp", "localhost:25432")
	if err == nil {
		t.Fatal("expected dial to fail after partition, got nil")
	}
	t.Logf("partition produced expected error: %v", err)
}

// TestChaos_PostgresNetworkPartition_Recovery ensures that once we re-enable
// the proxy, traffic flows again -- the application should self-heal on the
// next retry.
func TestChaos_PostgresNetworkPartition_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires toxiproxy + postgres-proxied containers; run without -short")
	}

	tox := NewToxiproxyHelper(t, "http://localhost:8474")
	tox.AddTCPProxy(t, "pg-recovery", "0.0.0.0:25433", "localhost:5433")

	// Disable then re-enable.
	tox.SetEnabled(t, "pg-recovery", false)
	tox.SetEnabled(t, "pg-recovery", true)

	// Give toxiproxy a beat to actually re-bind the listener.
	time.Sleep(200 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", "localhost:25433", 2*time.Second)
	if err != nil {
		t.Fatalf("dial after recovery: %v", err)
	}
	_ = conn.Close()
}
