package port

import "context"

// === Existing types (preserved for backward compatibility) ===

// ToolRegistryClient is the original search-only interface.
type ToolRegistryClient interface {
	SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error)
}

// ToolMatch is a search result for a tool query.
type ToolMatch struct {
	Name        string
	Description string
	Score       float64
}

// === New types ===

// ToolInfo describes a registered tool for discovery.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // JSON Schema
}

// ToolCall represents a request to invoke a tool.
type ToolCall struct {
	ToolName string
	Input    []byte // JSON-encoded input
}

// ToolResult is the result of a tool invocation.
type ToolResult struct {
	Output []byte // JSON-encoded output
	Error  string
}

// ToolSearcher matches queries to tools (keyword + future embedding).
type ToolSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]ToolMatch, error)
	SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error) // backward-compat alias
	Register(ctx context.Context, info ToolInfo) error
	List(ctx context.Context) ([]ToolInfo, error)
}

// ToolRunner executes registered tools.
type ToolRunner interface {
	Run(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolRegistryClientFull combines search + execution (what Agent uses).
type ToolRegistryClientFull interface {
	ToolSearcher
	ToolRunner
}
