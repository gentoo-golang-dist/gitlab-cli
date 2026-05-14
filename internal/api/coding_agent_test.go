package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCodingAgent(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "no agent detected",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name:     "AI_AGENT takes priority over all others",
			envVars:  map[string]string{"AI_AGENT": "claude-code_2-1-140_agent", "CLAUDECODE": "1", "CURSOR_AGENT": "1"},
			expected: "claude-code_2-1-140_agent",
		},
		{
			name:     "AI_AGENT passes through raw value",
			envVars:  map[string]string{"AI_AGENT": "some-custom-agent"},
			expected: "some-custom-agent",
		},
		{
			name:     "Claude Code",
			envVars:  map[string]string{"CLAUDECODE": "1"},
			expected: "claude-code",
		},
		{
			name:     "CLAUDECODE wrong value ignored",
			envVars:  map[string]string{"CLAUDECODE": "true"},
			expected: "",
		},
		{
			name:     "Codex CLI",
			envVars:  map[string]string{"CODEX_THREAD_ID": "thread_abc123"},
			expected: "codex",
		},
		{
			name:     "OpenCode",
			envVars:  map[string]string{"OPENCODE": "1"},
			expected: "opencode",
		},
		{
			name:     "Cursor",
			envVars:  map[string]string{"CURSOR_AGENT": "1"},
			expected: "cursor",
		},
		{
			name:     "AI_AGENT with spaces is ignored",
			envVars:  map[string]string{"AI_AGENT": "has spaces", "CLAUDECODE": "1"},
			expected: "claude-code",
		},
		{
			name:     "AI_AGENT with special chars is ignored",
			envVars:  map[string]string{"AI_AGENT": "agent/name!@#"},
			expected: "",
		},
		{
			name:     "AI_AGENT exceeding max length is ignored",
			envVars:  map[string]string{"AI_AGENT": "abcdefghijklmnopqrstuvwxyz-ABCDEFGHIJKLMNOPQRSTUVWXYZ-0123456789XYZ"},
			expected: "",
		},
		{
			name:     "AI_AGENT at exactly max length is accepted",
			envVars:  map[string]string{"AI_AGENT": "abcdefghijklmnopqrstuvwxyz-ABCDEFGHIJKLMNOPQRSTUVWXYZ-0123456789"},
			expected: "abcdefghijklmnopqrstuvwxyz-ABCDEFGHIJKLMNOPQRSTUVWXYZ-0123456789",
		},
	}

	allAgentVars := []string{
		"AI_AGENT", "CLAUDECODE", "CODEX_THREAD_ID",
		"OPENCODE", "CURSOR_AGENT",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range allAgentVars {
				t.Setenv(key, "")
			}
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			result := DetectCodingAgent()
			assert.Equal(t, tt.expected, result)
		})
	}
}
