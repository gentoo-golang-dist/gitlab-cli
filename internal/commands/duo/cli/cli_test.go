//go:build !integration

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd_Help(t *testing.T) {
	t.Parallel()

	// This test verifies that the command is properly configured for transparent
	// pass-through behavior, including --help flag handling. We test the command
	// structure without executing the duo binary to avoid download prompts.

	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	factory := cmdtest.NewTestFactory(ios)
	cmd := NewCmd(factory)

	// Verify DisableFlagParsing is enabled for transparent pass-through
	assert.True(t, cmd.DisableFlagParsing, "DisableFlagParsing should be enabled for transparent pass-through")

	// Verify the command accepts arbitrary arguments (no Args validator)
	assert.Nil(t, cmd.Args, "Args should be nil to accept any arguments")

	// Verify RunE is set (the function that handles help transformation)
	assert.NotNil(t, cmd.RunE, "RunE should be set to handle flag transformation")
}

func TestShouldForceUpdateCheck(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "env var set to true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "env var set to false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "env var not set",
			envValue: "",
			expected: false,
		},
		{
			name:     "env var set to other value",
			envValue: "yes",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GLAB_DUO_CLI_CHECK_UPDATE", tt.envValue)
			result := shouldForceUpdateCheck()
			assert.Equal(t, tt.expected, result)
		})
	}
}
