package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDependencyWarmer_Hash_Deterministic verifies that the same
// (projectID, runtime) inputs always produce the same sha256 hex digest.
func TestDependencyWarmer_Hash_Deterministic(t *testing.T) {
	w := NewDependencyWarmer("/tmp/of-deps")
	h1 := w.Hash("p-1", "node-react")
	h2 := w.Hash("p-1", "node-react")
	require.Equal(t, h1, h2, "Hash must be deterministic for identical inputs")
}

// TestDependencyWarmer_Hash_DifferentProjects verifies that different
// projectIDs produce different hashes.
func TestDependencyWarmer_Hash_DifferentProjects(t *testing.T) {
	w := NewDependencyWarmer("/tmp/of-deps")
	h1 := w.Hash("p-1", "node-react")
	h2 := w.Hash("p-2", "node-react")
	require.NotEqual(t, h1, h2, "different projectIDs must yield different hashes")
}
