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
)
