// Package mcpannotations defines constants for MCP command annotations
package mcpannotations

// MCP annotation keys for command classification
const (
	// Destructive marks commands that modify state (create, update, delete operations)
	Destructive = "mcp:destructive"
	// Safe marks commands that only read data (list, view, get operations)
	Safe = "mcp:safe"
	// Interactive marks commands that require an interactive TTY and should be excluded from MCP
	Interactive = "mcp:interactive"
	// Exclude marks commands that should never be exposed via MCP
	Exclude = "mcp:exclude"
)

// HasAnnotation checks if the given annotations map contains any valid MCP annotation.
// Returns false if annotations is nil or contains no MCP annotations.
// Commands without MCP annotations should not be exposed as MCP tools.
func HasAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	_, hasSafe := annotations[Safe]
	_, hasDestructive := annotations[Destructive]
	_, hasExclude := annotations[Exclude]
	_, hasInteractive := annotations[Interactive]
	return hasSafe || hasDestructive || hasExclude || hasInteractive
}
