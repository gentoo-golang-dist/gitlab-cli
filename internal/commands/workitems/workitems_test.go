package workitems

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdWorkItems(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := cmdtest.TestIOStreams()
	factory := cmdtest.NewTestFactory(ios)

	cmd := NewCmdWorkItems(factory)

	assert.NotNil(t, cmd)
	assert.Equal(t, "work-items <command> [flags]", cmd.Use)
	assert.Equal(t, "Manage work items. (EXPERIMENTAL)", cmd.Short)
	assert.True(t, cmd.HasSubCommands())

	// Check that list subcommand is present
	subcommands := cmd.Commands()
	subcommandNames := make([]string, len(subcommands))
	for i, subcmd := range subcommands {
		subcommandNames[i] = subcmd.Name()
	}

	assert.Contains(t, subcommandNames, "list")
}
