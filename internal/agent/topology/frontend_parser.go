// Package topology provides monorepo topology analysis for OpenForge.
//
// The package walks a monorepo on disk and builds a 3-level graph of
// frontend exports, backend functions, and the cross-stack links between
// them. The "level" classification (L1 = business / hooks, L2 = components,
// L3 = data) is a coarse-grained filter used by the UI to toggle views.
package topology

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FrontendNode represents a TypeScript / TSX export that was extracted
// from a source file during a monorepo walk.
type FrontendNode struct {
	File    string   `json:"file"`
	Export  string   `json:"export"`
	Type    string   `json:"type"` // "component" | "hook" | "util"
	Imports []string `json:"imports,omitempty"`
	Level   int      `json:"level"` // 1 = hook/business, 2 = default, 3 = data
}

// FrontendParser extracts export declarations from .ts / .tsx files.
//
// The implementation uses a deliberately small set of regexes — it is
// not a full TS parser. This is by design: the topology view is meant to
// give engineers an at-a-glance map of a monorepo, not to perform
// semantic analysis. A complete TS parser (e.g. tree-sitter) is overkill
// for the use case and would make the binary much harder to ship.
type FrontendParser struct{}

// exportDeclRE matches `export const Foo`, `export function Bar`, or
// `export default function Baz` and captures the exported identifier.
var exportDeclRE = regexp.MustCompile(`export\s+(?:default\s+)?(?:const|function|async\s+function)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

// Parse walks projectPath recursively and returns all frontend exports
// found. Files that cannot be read are silently skipped so that a single
// permission error does not abort the whole walk.
func (p *FrontendParser) Parse(projectPath string) ([]FrontendNode, error) {
	var nodes []FrontendNode
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Best-effort walk: skip unreadable entries.
			return nil
		}
		if info == nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".tsx" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, m := range exportDeclRE.FindAllStringSubmatch(string(content), -1) {
			name := m[1]
			nodes = append(nodes, FrontendNode{
				File:   path,
				Export: name,
				Type:   classifyType(name),
				Level:  classifyLevel(name),
			})
		}
		return nil
	})
	return nodes, err
}

// classifyType picks one of the coarse frontend kinds based on the
// exported name. Hooks (useFoo) are reported as "hook", data helpers as
// "util", and everything else as "component".
func classifyType(name string) string {
	if strings.HasPrefix(name, "use") && len(name) > 3 && isUpper(rune(name[3])) {
		return "hook"
	}
	if strings.HasPrefix(strings.ToLower(name), "data") {
		return "util"
	}
	return "component"
}

// classifyLevel assigns the L1/L2/L3 bucket used by the UI filter:
//   - L1: hooks (business layer) — anything that starts with "use" and
//     has an uppercase next character (matches React conventions).
//   - L3: data layer — names that contain "data" (case-insensitive).
//   - L2: everything else (default).
func classifyLevel(name string) int {
	if strings.HasPrefix(name, "use") && len(name) > 3 && isUpper(rune(name[3])) {
		return 1
	}
	if strings.Contains(strings.ToLower(name), "data") {
		return 3
	}
	return 2
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}
