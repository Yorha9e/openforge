package adapter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
)

// DependencyWarmer runs a script to pre-populate the dependency layer for
// a (projectID, runtime) pair. The layer is content-addressed by sha256 of
// "projectID:runtime" so repeated warmups for the same pair are idempotent.
type DependencyWarmer struct {
	layerDir string
	script   string
}

// NewDependencyWarmer wires a warmer that will execute `script` with the
// destination layer path as its first argument.
func NewDependencyWarmer(layerDir string) *DependencyWarmer {
	return &DependencyWarmer{
		layerDir: layerDir,
		script:   "scripts/warm_dependencies.sh",
	}
}

// Warm executes the warmup script targeting the (projectID, runtime) layer.
// The destination directory is created and populated with pre-installed
// toolchain dependencies (e.g. react@19, typescript@5).
func (w *DependencyWarmer) Warm(ctx context.Context, projectID, runtime string) error {
	layerPath := w.layerPath(projectID, runtime)
	cmd := exec.CommandContext(ctx, "/bin/bash", w.script, layerPath)
	return cmd.Run()
}

// Hash returns the sha256 hex digest of "projectID:runtime". Callers can use
// this to deduplicate layer directories or to invalidate caches.
func (w *DependencyWarmer) Hash(projectID, runtime string) string {
	sum := sha256.Sum256([]byte(projectID + ":" + runtime))
	return fmt.Sprintf("%x", sum)
}

// layerPath is the on-disk directory for a given (projectID, runtime) pair.
func (w *DependencyWarmer) layerPath(projectID, runtime string) string {
	return filepath.Join(w.layerDir, w.Hash(projectID, runtime))
}
