//go:build !integration

package cli

import (
	"testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"

	"github.com/stretchr/testify/assert"
)

func TestNewCmdCli(t *testing.T) {
	exec := cmdtest.SetupCmdForTest(t, NewCmdCli, false)

	// Get the command by executing with --help
	output, err := exec("--help")

	// The command should show help without error
	assert.NoError(t, err)

	stdout := output.String()
	assert.Contains(t, stdout, "GitLab Duo CLI")
	assert.Contains(t, stdout, "npx")
	assert.Contains(t, stdout, "Node.js version 22")
}
