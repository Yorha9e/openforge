// 4.4 -- Docker daemon down.
//
// Simulates the docker daemon being unreachable by pointing the docker client
// at a TCP listener that immediately closes the connection.  The OpenForge
// sandbox allocator depends on moby/docker; we want a clean, fast error
// rather than a hang when the daemon is gone.
//
// This test does NOT spin up toxiproxy -- a stub listener is cheaper and
// conveys the same signal: the dependency disappeared.
package chaos

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestChaos_DockerDaemonDown_AllocatorFailsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("does not require toxiproxy, but exercises docker client")
	}

	// 1. Stand up a stub docker "daemon" that just 500s and closes.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the daemon going away mid-request.
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijack", http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer stub.Close()

	// 2. Build a docker client pointed at the stub.  We use WithHostFromClient
	//    so the client trusts the stub's (self-signed) transport.
	httpClient := &http.Client{Timeout: 500 * time.Millisecond}
	apiClient, err := client.NewClientWithOpts(client.WithHost(stub.URL), client.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("new docker client: %v", err)
	}

	// 3. A list-containers call should fail fast (well under 2s) -- not hang
	//    waiting for the daemon to come back.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err = apiClient.ContainerList(ctx, client.ContainerListOptions{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from docker client when daemon is down, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("docker client took %v to fail; expected fast failure", elapsed)
	}
	t.Logf("docker client failed in %v with: %v", elapsed, err)
}

// TestChaos_DockerDaemonDown_RefusedConnection covers the case where the
// docker socket port is closed entirely (daemon process exited, not just
// stalled).  The client should report a connection-refused error.
func TestChaos_DockerDaemonDown_RefusedConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("does not require toxiproxy, but exercises docker client")
	}

	// Bind + immediately close to get a port that's almost certainly free.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	apiClient, err := client.NewClientWithOpts(client.WithHost("tcp://"+addr))
	if err != nil {
		t.Fatalf("new docker client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err = apiClient.ContainerList(ctx, client.ContainerListOptions{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("connection took %v to fail; expected fast failure", elapsed)
	}
	t.Logf("refused-connection in %v: %v", elapsed, err)
}
