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
			name:     "Codex CLI",
			envVars:  map[string]string{"CODEX_THREAD_ID": "thread_abc123"},
			expected: "codex",
		},
		{
			name:     "Cline",
			envVars:  map[string]string{"CLINE_ACTIVE": "true"},
			expected: "cline",
		},
		{
			name:     "Amazon Q",
			envVars:  map[string]string{"AWS_EXECUTION_ENV": "AmazonQ-For-CLI Version/1.0"},
			expected: "amazon-q",
		},
		{
			name:     "Amazon Q appended to existing value",
			envVars:  map[string]string{"AWS_EXECUTION_ENV": "Lambda AmazonQ-For-CLI"},
			expected: "amazon-q",
		},
		{
			name:     "CLAUDECODE fallback when no higher priority match",
			envVars:  map[string]string{"CLAUDECODE": "1"},
			expected: "claude-code",
		},
		{
			name:     "CLAUDECODE wrong value ignored",
			envVars:  map[string]string{"CLAUDECODE": "true"},
			expected: "",
		},
		{
			name:     "CLINE_ACTIVE wrong value ignored",
			envVars:  map[string]string{"CLINE_ACTIVE": "1"},
			expected: "",
		},
	}

	allAgentVars := []string{
		"AI_AGENT", "OPENCODE", "CURSOR_AGENT",
		"CODEX_THREAD_ID", "CLINE_ACTIVE", "AWS_EXECUTION_ENV",
		"CLAUDECODE",
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
