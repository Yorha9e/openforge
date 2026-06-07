// Package chaos contains fault-injection tests built on top of toxiproxy.
//
// Toxiproxy is a TCP proxy that sits between the test client and the upstream
// service (postgres, redis, docker, etc).  Each test creates a proxy via the
// admin API, drives a real connection through it, and then injects toxics
// (latency, partition, bandwidth, etc.) to validate that the application
// degrades gracefully.
//
// The helpers in this file are intentionally minimal -- they only wrap the
// bits of the client API that the chaos scenarios need.  When the toxiproxy
// client gains new toxic types we extend them here.
package chaos

import (
	"net"
	"sync"
	"testing"
	"time"

	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

// ToxiproxyHelper wraps the toxiproxy admin client and remembers proxies /
// toxics it created so they can be torn down via t.Cleanup.
type ToxiproxyHelper struct {
	Client   *toxiproxy.Client
	AdminURL string
}

// NewToxiproxyHelper builds a helper pointed at the toxiproxy admin API.  In
// docker-compose the admin port is published on localhost:8474.
func NewToxiproxyHelper(t *testing.T, adminURL string) *ToxiproxyHelper {
	t.Helper()
	return &ToxiproxyHelper{
		Client:   toxiproxy.NewClient(adminURL),
		AdminURL: adminURL,
	}
}

// AddTCPProxy creates a new proxy and registers a cleanup hook to delete it
// when the test ends.  `listen` is the address toxiproxy itself binds to
// (something reachable from the test process), `upstream` is the real
// service address inside the docker network.
func (h *ToxiproxyHelper) AddTCPProxy(t *testing.T, name, listen, upstream string) *toxiproxy.Proxy {
	t.Helper()
	proxy, err := h.Client.CreateProxy(name, listen, upstream)
	if err != nil {
		t.Fatalf("CreateProxy %s (listen=%s upstream=%s): %v", name, listen, upstream, err)
	}
	t.Cleanup(func() {
		// best-effort delete; ignore error since toxiproxy itself may be gone
		_ = proxy.Delete()
	})
	return proxy
}

// SetEnabled flips a proxy on or off.  SetEnabled(false) is how the network
// partition test cuts connectivity.
func (h *ToxiproxyHelper) SetEnabled(t *testing.T, name string, enabled bool) {
	t.Helper()
	proxy, err := h.Client.Proxy(name)
	if err != nil {
		t.Fatalf("Proxy(%s): %v", name, err)
	}
	if enabled {
		if err := proxy.Enable(); err != nil {
			t.Fatalf("Enable %s: %v", name, err)
		}
		return
	}
	if err := proxy.Disable(); err != nil {
		t.Fatalf("Disable %s: %v", name, err)
	}
}

// AddLatency injects a `latency` toxic with the given delay and 100% toxicity
// on the downstream stream (toxiproxy -> client) -- the same direction the
// application reads its responses from.  A cleanup hook removes it.
func (h *ToxiproxyHelper) AddLatency(t *testing.T, name string, latency time.Duration) {
	t.Helper()
	proxy, err := h.Client.Proxy(name)
	if err != nil {
		t.Fatalf("Proxy(%s): %v", name, err)
	}
	toxicName := "latency-down-" + name
	if _, err := proxy.AddToxic(toxicName, "latency", "downstream", 1.0, toxiproxy.Attributes{
		"latency": latency.Milliseconds(),
	}); err != nil {
		t.Fatalf("AddToxic latency %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = proxy.RemoveToxic(toxicName)
	})
}

// AddBandwidthLimit throttles a proxy to `kbps` kilobits per second.  Used by
// the slow-network chaos test.
func (h *ToxiproxyHelper) AddBandwidthLimit(t *testing.T, name string, kbps int64) {
	t.Helper()
	proxy, err := h.Client.Proxy(name)
	if err != nil {
		t.Fatalf("Proxy(%s): %v", name, err)
	}
	toxicName := "bandwidth-down-" + name
	if _, err := proxy.AddToxic(toxicName, "bandwidth", "downstream", 1.0, toxiproxy.Attributes{
		"rate": kbps,
	}); err != nil {
		t.Fatalf("AddToxic bandwidth %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = proxy.RemoveToxic(toxicName)
	})
}

// AddTimeout drops the connection after `timeoutMs` milliseconds without
// activity, simulating half-open sockets caused by NAT timeouts.
func (h *ToxiproxyHelper) AddTimeout(t *testing.T, name string, timeoutMs int) {
	t.Helper()
	proxy, err := h.Client.Proxy(name)
	if err != nil {
		t.Fatalf("Proxy(%s): %v", name, err)
	}
	toxicName := "timeout-down-" + name
	if _, err := proxy.AddToxic(toxicName, "timeout", "downstream", 1.0, toxiproxy.Attributes{
		"timeout": timeoutMs,
	}); err != nil {
		t.Fatalf("AddToxic timeout %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = proxy.RemoveToxic(toxicName)
	})
}

// netListenLoopback stands up a tiny TCP echo server on a free loopback port
// and returns the listener.  The caller is responsible for Close().  Used by
// the latency and slow-redis chaos tests so we don't need a real redis
// process on the host.
func netListenLoopback(t *testing.T) (net.Listener, error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
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
				buf := make([]byte, 4096)
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
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
	})
	return ln, nil
}
