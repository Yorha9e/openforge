// Package contract holds golden-file-based gRPC contract tests that
// validate Go↔Node proto wire-format compatibility.
//
// The contract is simple: a canonical JSON (snake_case, protojson-style)
// is stored on disk in test/contract/golden/ and mirrored at
// nodejs-io/src/contract/golden/. Both runtimes must be able to
// (un)marshal the same JSON to the same proto message.
package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadGolden returns the bytes of a golden file relative to the
// test/contract directory. e.g. loadGolden(t, "golden/llm_chat.req.json").
func loadGolden(t *testing.T, relPath string) []byte {
	t.Helper()

	// Walk up to test/contract/. The golden files live next to the tests.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	dir := filepath.Dir(thisFile)

	full := filepath.Join(dir, relPath)
	b, err := os.ReadFile(full)
	require.NoError(t, err, "read golden %s", relPath)
	return b
}
