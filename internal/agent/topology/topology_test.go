package topology

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFrontendParser_ExtractsComponentNames verifies that a basic
// `export const Foo = ...` declaration is recognized as a single node
// and that the export name is captured verbatim.
func TestFrontendParser_ExtractsComponentNames(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "Foo.tsx")
	require.NoError(t, os.WriteFile(tsFile, []byte("export const Foo = () => <div/>"), 0o644))

	p := &FrontendParser{}
	nodes, err := p.Parse(tmpDir)
	require.NoError(t, err)
	require.NotEmpty(t, nodes, "expected at least one node to be extracted")
	require.Equal(t, "Foo", nodes[0].Export)
}

// TestFrontendParser_ClassifyLevel documents the L1/L2/L3 semantics:
//   - "useFoo" → L1 (hook / business layer)
//   - "dataXxx" → L3 (data layer)
//   - other     → L2 (default)
func TestFrontendParser_ClassifyLevel(t *testing.T) {
	require.Equal(t, 1, classifyLevel("useFoo"))
	require.Equal(t, 3, classifyLevel("dataFetcher"))
	require.Equal(t, 3, classifyLevel("DataGrid"))
	require.Equal(t, 2, classifyLevel("Card"))
}

// TestBackendParser_ExtractsGoFuncs verifies that a top-level
// `func Bar()` declaration is recognized as a single node.
func TestBackendParser_ExtractsGoFuncs(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "foo.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package foo\n\nfunc Bar() {}\n"), 0o644))

	p := &BackendParser{}
	nodes, err := p.Parse(tmpDir)
	require.NoError(t, err)
	require.NotEmpty(t, nodes, "expected at least one Go function node to be extracted")
	require.Equal(t, "Bar", nodes[0].Name)
}

// TestCrossStackLinker_LinksByName verifies that a frontend export
// and a backend function with the same identifier are linked together.
func TestCrossStackLinker_LinksByName(t *testing.T) {
	frontends := []FrontendNode{
		{File: "Foo.tsx", Export: "Foo", Type: "component", Level: 2},
	}
	backends := []BackendNode{
		{File: "foo.go", Name: "Foo"},
	}
	links := Link(frontends, backends)
	require.Len(t, links, 1, "expected exactly one naming-match link")
	require.Equal(t, "naming_match", links[0].Type)
}
