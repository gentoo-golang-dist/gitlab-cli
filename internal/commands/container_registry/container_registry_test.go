//go:build !integration

package container_registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(cmdtest.NewTestFactory(nil))

	assert.Equal(t, "container-registry <command> [flags]", cmd.Use)
	assert.Empty(t, cmd.Aliases)
	repositoryCmd, _, err := cmd.Find([]string{"repository"})
	assert.NoError(t, err)
	assert.Equal(t, "repository <command> [flags]", repositoryCmd.Use)
	tagCmd, _, err := cmd.Find([]string{"tag"})
	assert.NoError(t, err)
	assert.Equal(t, "tag <command> [flags]", tagCmd.Use)
}
