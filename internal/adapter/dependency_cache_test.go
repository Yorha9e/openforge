package adapter

import (
	"os"
	"testing"
)

func TestDependencyCache_LayerIsDeterministic(t *testing.T) {
	dc := NewDependencyCache("/tmp/cache")
	spec := DependencySpec{ProjectID: "proj-1", Runtime: "node", LockfileHash: "hash-123"}
	
	layer1 := dc.Layer(spec)
	layer2 := dc.Layer(spec)
	
	if layer1 != layer2 {
		t.Fatalf("expected deterministic layer path, got %s and %s", layer1, layer2)
	}
}

func TestDependencyCache_WarmCreatesLayerDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dc := NewDependencyCache(tmpDir)
	spec := DependencySpec{ProjectID: "proj-1", Runtime: "node", LockfileHash: "hash-123"}
	
	dir, err := dc.Warm(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !fi.IsDir() {
		t.Fatal("expected a directory")
	}
}
