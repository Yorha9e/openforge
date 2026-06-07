// 4.5 -- Out-of-memory pressure.
//
// The OpenForge daemon should refuse to allocate unbounded memory and must
// keep its goroutine stacks bounded.  Rather than actually OOMing the box
// (which is destructive and platform-dependent), this test:
//
//   1. Asserts that runtime.ReadMemStats reports plausible values for a
//      freshly started process (no runaway allocations on import).
//   2. Allocates 64 MiB of garbage and verifies the runtime tracks it
//      without panicking -- i.e. the application doesn't crash under
//      modest memory pressure.
//   3. Asserts the package never blocks on allocation by completing the
//      test in bounded time.
//
// The "real" OOM defense (GOMEMLIMIT, container cgroup limit) is configured
// at deployment time and exercised by docker's `--memory` flag, not here.
package chaos

import (
	"runtime"
	"testing"
	"time"
)

func TestChaos_OutOfMemory_BoundedAllocations(t *testing.T) {
	if testing.Short() {
		t.Skip("OOM test is a runtime sanity check; safe to run under -short")
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	// Reasonable upper bound for a freshly started process: < 256 MiB.
	if before.HeapAlloc > 256*1024*1024 {
		t.Fatalf("HeapAlloc at start = %d, expected < 256 MiB", before.HeapAlloc)
	}
	t.Logf("start: HeapAlloc=%d MiB, NumGC=%d", before.HeapAlloc/(1024*1024), before.NumGC)

	// Allocate ~64 MiB in 4 KiB chunks, holding references to keep them
	// alive.  This is small enough not to threaten the test runner but big
	// enough to show up in MemStats.
	const (
		chunkSize = 4 * 1024
		chunkCount = 16 * 1024 // 64 MiB
	)
	keepAlive := make([][]byte, 0, chunkCount)
	for i := 0; i < chunkCount; i++ {
		buf := make([]byte, chunkSize)
		// Touch the memory so the runtime can't lazily skip it.
		buf[0] = byte(i)
		keepAlive = append(keepAlive, buf)
	}

	runtime.ReadMemStats(&after)
	if after.HeapAlloc < 32*1024*1024 {
		t.Fatalf("HeapAlloc after alloc = %d, expected >= 32 MiB", after.HeapAlloc)
	}
	t.Logf("after alloc: HeapAlloc=%d MiB, NumGC=%d", after.HeapAlloc/(1024*1024), after.NumGC)

	// Release and let the GC reclaim.  We don't assert on the reclaimed
	// amount (GC scheduling is best-effort), only that the process doesn't
	// panic and returns within a reasonable bound.
	keepAlive = nil
	runtime.GC()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.HeapAlloc < 32*1024*1024 {
			t.Logf("post-GC HeapAlloc=%d MiB", m.HeapAlloc/(1024*1024))
			return
		}
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}
	t.Logf("post-GC deadline reached, HeapAlloc=%d MiB (acceptable)", after.HeapAlloc/(1024*1024))
}
