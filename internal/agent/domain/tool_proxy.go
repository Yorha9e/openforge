package domain

// ToolProxyRequest represents a request from the server to the client to execute a tool.
// This is sent via WebSocket from server to Electron client.
type ToolProxyRequest struct {
	RequestID string         `json:"request_id"`
	Tool      string         `json:"tool"`
	Params    map[string]any `json:"params"`
}

// ToolProxyResult represents the result of a tool execution on the client.
// This is sent via WebSocket from Electron client back to server.
type ToolProxyResult struct {
	RequestID string         `json:"request_id"`
	Success   bool           `json:"success"`
	Data      map[string]any `json:"data,omitempty"`
	Error     *ProxyError    `json:"error,omitempty"`
}

// ProxyError represents an error that occurred during tool proxy execution.
type ProxyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// IsLocalPath checks if a path is a local path that should be proxied to the client.
// Local paths are those that don't start with /workspace/ or other server path markers.
func IsLocalPath(path string) bool {
	if path == "" {
		return false
	}
	// Server paths typically start with /workspace/ or are absolute paths
	// that exist on the server
	if len(path) > 11 && path[:11] == "/workspace/" {
		return false
	}
	// Windows paths
	if len(path) >= 2 && path[1] == ':' {
		return true
	}
	// Unix paths that don't start with /workspace/
	if path[0] == '/' && (len(path) < 11 || path[:11] != "/workspace/") {
		return true
	}
	// Relative paths
	if path[0] != '/' {
		return true
	}
	return false
}