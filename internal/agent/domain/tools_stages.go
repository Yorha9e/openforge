// tools_stages.go
// Phase 7: replace hardcoded StageToolMap with ToolRegistry dynamic query

package domain

import (
	"fmt"
	"strings"
)

// ToolDefinition tool definition aligned with port.ToolInfo for Phase 7 migration
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // aligned with port.ToolInfo
	ReadOnly    bool
}

// StageToolMap hardcoded tool list organized by stage
var StageToolMap = map[string][]ToolDefinition{
	"clarify": {
		{Name: "read_file", Description: "Read file contents", ReadOnly: true},
		{Name: "search_content", Description: "Search with regex", ReadOnly: true},
		{Name: "analyze_topology", Description: "Analyze project topology", ReadOnly: true},
		{Name: "lsp_symbols", Description: "Get document symbols", ReadOnly: true},
	},
	"decompose": {
		{Name: "read_file", Description: "Read file contents", ReadOnly: true},
		{Name: "search_content", Description: "Search with regex", ReadOnly: true},
		{Name: "analyze_topology", Description: "Analyze project topology", ReadOnly: true},
		{Name: "lsp_references", Description: "Find all references", ReadOnly: true},
	},
	"implement": {
		{Name: "acquire_file_lock", Description: "Acquire lock before modification", ReadOnly: false},
		{Name: "release_file_lock", Description: "Release file lock", ReadOnly: false},
		{Name: "read_file", Description: "Read file contents", ReadOnly: true},
		{Name: "edit_file", Description: "Edit existing file", ReadOnly: false},
		{Name: "write_file", Description: "Write new file", ReadOnly: false},
		{Name: "bash", Description: "Execute shell command", ReadOnly: false},
		{Name: "lsp_hover", Description: "Get symbol info", ReadOnly: true},
		{Name: "lsp_definition", Description: "Go to definition", ReadOnly: true},
		{Name: "lsp_references", Description: "Find all references", ReadOnly: true},
	},
	"test": {
		{Name: "read_file", Description: "Read file contents", ReadOnly: true},
		{Name: "edit_file", Description: "Edit file to fix failures", ReadOnly: false},
		{Name: "bash", Description: "Run tests and lint", ReadOnly: false},
		{Name: "search_content", Description: "Search test patterns", ReadOnly: true},
	},
	"deploy": {
		{Name: "bash", Description: "Run deployment commands", ReadOnly: false},
		{Name: "read_file", Description: "Read deployment logs", ReadOnly: true},
		{Name: "manage_sandbox", Description: "Manage deployment sandbox", ReadOnly: false},
	},
	"verify": {
		{Name: "read_file", Description: "Read changed files", ReadOnly: true},
		{Name: "bash", Description: "Run verification scripts", ReadOnly: false},
		{Name: "write_knowledge_delta", Description: "Write learned preferences", ReadOnly: false},
	},
}

// PermissionFilter allowed tool whitelist in plan mode
var PermissionFilter = map[string][]string{
	"plan": {
		"read_file", "search_content", "search_files",
		"analyze_topology", "lsp_hover", "lsp_symbols",
		"lsp_references", "lsp_definition",
		"list_models", "check_token_budget",
		"query_module_ownership", "validate_artifact_hash",
		"generate_artifact_url",
	},
}

// InjectTools returns tool description text and definitions for a stage
func InjectTools(stage, permissionMode string) (string, []ToolDefinition) {
	tools := StageToolMap[stage]
	if tools == nil {
		return "", nil
	}
	if permissionMode == "plan" {
		tools = filterByPermission(tools)
	}
	return buildToolDescription(tools), tools
}

func filterByPermission(tools []ToolDefinition) []ToolDefinition {
	allowed := PermissionFilter["plan"]
	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}
	var filtered []ToolDefinition
	for _, t := range tools {
		if allowedSet[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func buildToolDescription(tools []ToolDefinition) string {
	var b strings.Builder
	b.WriteString("<available_tools>\n")
	for _, t := range tools {
		b.WriteString(fmt.Sprintf("<tool name=\"%s\">%s</tool>\n", t.Name, t.Description))
	}
	b.WriteString("</available_tools>\n")
	return b.String()
}
