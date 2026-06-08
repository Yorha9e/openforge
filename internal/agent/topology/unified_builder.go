package topology

// Graph is the 3-level topology graph returned by Analyze. The shape is
// intentionally flat: the frontend can group/filter by Level in a single
// pass without re-deriving it.
type Graph struct {
	Frontends []FrontendNode `json:"frontends"`
	Backends  []BackendNode  `json:"backends"`
	Links     []NodeLink     `json:"links"`
}

// UnifiedBuilder stitches the parser outputs into a Graph. Keeping this
// step in its own type makes it easy to add caching or to run the
// sub-parsers in parallel later.
type UnifiedBuilder struct {
	Frontend *FrontendParser
	Backend  *BackendParser
}

// NewUnifiedBuilder returns a builder with default parser instances.
func NewUnifiedBuilder() *UnifiedBuilder {
	return &UnifiedBuilder{
		Frontend: &FrontendParser{},
		Backend:  &BackendParser{},
	}
}

// Build runs the frontend + backend parsers and the cross-stack linker
// in sequence. It returns the first non-nil parser error it encounters.
func (b *UnifiedBuilder) Build(projectPath string) (*Graph, error) {
	fes, err := b.Frontend.Parse(projectPath)
	if err != nil {
		return nil, err
	}
	bes, err := b.Backend.Parse(projectPath)
	if err != nil {
		return nil, err
	}
	links := Link(fes, bes)
	if fes == nil {
		fes = []FrontendNode{}
	}
	if bes == nil {
		bes = []BackendNode{}
	}
	if links == nil {
		links = []NodeLink{}
	}
	return &Graph{Frontends: fes, Backends: bes, Links: links}, nil
}

// Analyze is the package-level convenience entry point used by the
// HTTP handler in internal/server/routes.go.
func Analyze(projectPath string) (*Graph, error) {
	return NewUnifiedBuilder().Build(projectPath)
}
