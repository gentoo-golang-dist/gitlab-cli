package api

import "os"

func DetectCodingAgent() string {
	if v := os.Getenv("AI_AGENT"); v != "" {
		return v
	}
	if os.Getenv("CLAUDECODE") == "1" {
		return "claude-code"
	}
	if os.Getenv("CODEX_THREAD_ID") != "" {
		return "codex"
	}
	if os.Getenv("OPENCODE") == "1" {
		return "opencode"
	}
	if os.Getenv("CURSOR_AGENT") == "1" {
		return "cursor"
	}
	return ""
}
