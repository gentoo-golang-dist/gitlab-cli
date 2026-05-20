//go:build !integration

package tag

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(cmdtest.NewTestFactory(nil))

	assert.Equal(t, "tag <command> [flags]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "tags")
	listCmd, _, err := cmd.Find([]string{"list"})
	assert.NoError(t, err)
	assert.Equal(t, "list <repository-id> [flags]", listCmd.Use)
	viewCmd, _, err := cmd.Find([]string{"view"})
	assert.NoError(t, err)
	assert.Equal(t, "view <repository-id> <tag-name> [flags]", viewCmd.Use)
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	assert.NoError(t, err)
	assert.Equal(t, "delete <repository-id> <tag-name> [flags]", deleteCmd.Use)
	deleteTagsCmd, _, err := cmd.Find([]string{"delete-tags"})
	assert.NoError(t, err)
	assert.Equal(t, "delete-tags <repository-id> [flags]", deleteTagsCmd.Use)
}
