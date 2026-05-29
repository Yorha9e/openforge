package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ToolProxyExecutor implements the tool.Executor interface for tools that need
// to be executed on the client (Electron) via WebSocket proxy.
type ToolProxyExecutor struct {
	rpc *WSRPC
}

// NewToolProxyExecutor creates a new ToolProxyExecutor with the given WSRPC instance.
func NewToolProxyExecutor(rpc *WSRPC) *ToolProxyExecutor {
	return &ToolProxyExecutor{
		rpc: rpc,
	}
}

// Execute executes a tool via the WebSocket proxy.
// It checks if the tool parameters contain a local path that should be proxied,
// and if so, sends a proxy request to the client.
func (e *ToolProxyExecutor) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Check if this is a local file operation that should be proxied
	if !e.shouldProxy(args) {
		return "", fmt.Errorf("tool should not be proxied")
	}
	
	// Generate a unique request ID
	requestID := uuid.New().String()
	
	// Get tool name from context or args
	toolName, _ := args["tool"].(string)
	if toolName == "" {
		toolName = "unknown"
	}
	
	// Create proxy request
	req := ToolProxyRequest{
		RequestID: requestID,
		Tool:      toolName,
		Params:    args,
	}
	
	// Send request and wait for response
	result, err := e.rpc.SendProxyRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("proxy request failed: %w", err)
	}
	
	// Check for errors in the result
	if !result.Success {
		if result.Error != nil {
			return "", fmt.Errorf("proxy error [%s]: %s", result.Error.Code, result.Error.Message)
		}
		return "", fmt.Errorf("proxy request failed with unknown error")
	}
	
	// Extract output from result data
	output, _ := result.Data["output"].(string)
	return output, nil
}

// shouldProxy checks if the tool execution should be proxied to the client.
// It examines the tool parameters to determine if they contain local paths.
func (e *ToolProxyExecutor) shouldProxy(args map[string]interface{}) bool {
	// Check for path parameter
	if path, ok := args["path"].(string); ok {
		return IsLocalPath(path)
	}
	
	// Check for file parameter
	if file, ok := args["file"].(string); ok {
		return IsLocalPath(file)
	}
	
	// Check for directory parameter
	if dir, ok := args["dir"].(string); ok {
		return IsLocalPath(dir)
	}
	
	// Check for filePath parameter
	if filePath, ok := args["filePath"].(string); ok {
		return IsLocalPath(filePath)
	}
	
	return false
}

// ProxyExecutorMeta returns a ToolMeta for a proxy executor.
// This can be used to register proxy tools in the tool registry.
func ProxyExecutorMeta(name string, proxy *ToolProxyExecutor) ToolMeta {
	return ToolMeta{
		Name:              name,
		IsConcurrencySafe: true,
		Timeout:           30 * time.Second,
		Executor:          proxy.Execute,
	}
}