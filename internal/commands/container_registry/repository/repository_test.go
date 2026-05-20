//go:build !integration

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(cmdtest.NewTestFactory(nil))

	assert.Equal(t, "repository <command> [flags]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "repositories")
	listCmd, _, err := cmd.Find([]string{"list"})
	assert.NoError(t, err)
	assert.Equal(t, "list [flags]", listCmd.Use)
	viewCmd, _, err := cmd.Find([]string{"view"})
	assert.NoError(t, err)
	assert.Equal(t, "view <repository-id> [flags]", viewCmd.Use)
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	assert.NoError(t, err)
	assert.Equal(t, "delete <repository-id> [flags]", deleteCmd.Use)
}
