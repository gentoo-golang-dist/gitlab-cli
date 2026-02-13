package mcpannotations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "empty annotations map",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name:        "only non-MCP annotations",
			annotations: map[string]string{"help:arguments": "some help", "other": "value"},
			expected:    false,
		},
		{
			name:        "has Safe annotation",
			annotations: map[string]string{Safe: "true"},
			expected:    true,
		},
		{
			name:        "has Destructive annotation",
			annotations: map[string]string{Destructive: "true"},
			expected:    true,
		},
		{
			name:        "has Exclude annotation",
			annotations: map[string]string{Exclude: "true"},
			expected:    true,
		},
		{
			name:        "has Interactive annotation",
			annotations: map[string]string{Interactive: "true"},
			expected:    true,
		},
		{
			name:        "has multiple MCP annotations",
			annotations: map[string]string{Safe: "true", Exclude: "true"},
			expected:    true,
		},
		{
			name:        "has MCP annotation with other annotations",
			annotations: map[string]string{Safe: "true", "help:arguments": "some help"},
			expected:    true,
		},
		{
			name:        "MCP annotation with false value still counts",
			annotations: map[string]string{Safe: "false"},
			expected:    true,
		},
		{
			name:        "MCP annotation with empty value still counts",
			annotations: map[string]string{Destructive: ""},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := HasAnnotation(tt.annotations)
			assert.Equal(t, tt.expected, result)
		})
	}
}
