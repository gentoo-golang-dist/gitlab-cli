package api

import (
	"os"
	"strings"
)

// DetectCodingAgent returns the name of the AI coding agent that invoked glab,
// based on well-known environment variables. Returns empty string if no agent
// is detected. Only includes agents with confirmed, reliable sentinel variables.
func DetectCodingAgent() string {
	if os.Getenv("CLAUDECODE") == "1" {
		return "claude-code"
	}
	if os.Getenv("OPENCODE") == "1" {
		return "opencode"
	}
	if os.Getenv("CURSOR_AGENT") == "1" {
		return "cursor"
	}
	if os.Getenv("CODEX_THREAD_ID") != "" {
		return "codex"
	}
	if os.Getenv("CLINE_ACTIVE") == "true" {
		return "cline"
	}
	if strings.Contains(os.Getenv("AWS_EXECUTION_ENV"), "AmazonQ-For-CLI") {
		return "amazon-q"
	}
	return ""
}
