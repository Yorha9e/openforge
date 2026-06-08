// Package topology — see frontend_parser.go for the package overview.
//
// This file is intentionally a thin re-export: the public entry point
// is Analyze() (defined in unified_builder.go) and a few helper types.
// Splitting it into its own file makes the public surface easy to
// find without scrolling through the implementation.
package topology

// Re-exports for convenience. External callers should usually just
// import this package and call Analyze() directly.
type (
	MonorepoNode = FrontendNode
	MonorepoLink = NodeLink
)
