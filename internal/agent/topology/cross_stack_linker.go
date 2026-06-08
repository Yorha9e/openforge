package topology

// NodeLink represents an inferred relationship between a frontend export
// and a backend function. The type is named NodeLink (not Link) so that
// the package can also expose a Link() helper function without a name
// collision.
type NodeLink struct {
	Frontend FrontendNode `json:"frontend"`
	Backend  BackendNode  `json:"backend"`
	Type     string       `json:"type"`
}

// Link walks the two slices and pairs up nodes that share the same
// export / function name. The algorithm is O(F*B) which is fine for
// monorepos of any practical size (hundreds, not millions, of entries).
func Link(frontends []FrontendNode, backends []BackendNode) []NodeLink {
	links := make([]NodeLink, 0)
	seen := make(map[string]struct{})
	for i := range frontends {
		fe := frontends[i]
		for j := range backends {
			be := backends[j]
			if fe.Export == "" || be.Name == "" {
				continue
			}
			if fe.Export != be.Name {
				continue
			}
			// Guard against duplicate links if the same name occurs
			// multiple times on either side — we still emit one link
			// per (file, name) pair, but the de-dup is cheap and
			// protects against pathological input.
			key := fe.File + "|" + be.File + "|" + fe.Export
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			links = append(links, NodeLink{Frontend: fe, Backend: be, Type: "naming_match"})
		}
	}
	return links
}
