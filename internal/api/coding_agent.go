package api

import (
	"os"
	"regexp"
)

var agentValueRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// AI_AGENT is the universal escape hatch; hardcoded agents are alphabetical, no priority implied.
func DetectCodingAgent() string {
	if v := os.Getenv("AI_AGENT"); v != "" && agentValueRE.MatchString(v) {
		return v
	}
	if os.Getenv("CLAUDECODE") == "1" {
		return "claude-code"
	}
	if os.Getenv("CODEX_THREAD_ID") != "" {
		return "codex"
	}
	if os.Getenv("CURSOR_AGENT") == "1" {
		return "cursor"
	}
	if os.Getenv("OPENCODE") == "1" {
		return "opencode"
	}
	return ""
}
