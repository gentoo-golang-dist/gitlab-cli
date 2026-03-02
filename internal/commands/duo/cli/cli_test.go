//go:build !integration

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd_Help(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
	)

	_, err := exec("--help")
	assert.NoError(t, err)
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
