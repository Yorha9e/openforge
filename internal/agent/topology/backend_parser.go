package topology

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BackendNode represents a top-level Go function that was extracted
// from a source file during a monorepo walk.
type BackendNode struct {
	File  string `json:"file"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// BackendParser extracts top-level `func Foo()` declarations from
// Go source files. It is a regex-based scanner — see FrontendParser
// for the rationale (we want a small, dependency-free topology view).
type BackendParser struct{}

// funcDeclRE matches a top-level `func Name(` declaration. We anchor
// at start-of-line so that `func` tokens appearing inside strings or
// comments are far less likely to be picked up. Multi-line
// declarations (rare for top-level funcs) are not supported.
var funcDeclRE = regexp.MustCompile(`(?m)^func\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// Parse walks projectPath and returns all backend function names.
func (p *BackendParser) Parse(projectPath string) ([]BackendNode, error) {
	var nodes []BackendNode
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".go") {
			return nil
		}
		// Skip generated files and test fixtures to keep the graph
		// focused on production code.
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".pb.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, m := range funcDeclRE.FindAllStringSubmatch(string(content), -1) {
			nodes = append(nodes, BackendNode{
				File:  path,
				Name:  m[1],
				Level: 2, // default; specialized heuristics live elsewhere
			})
		}
		return nil
	})
	return nodes, err
}
