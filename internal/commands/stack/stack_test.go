//go:build !integration

package stack

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/text"
	"gitlab.com/gitlab-org/cli/test"
)

func TestStackCmd(t *testing.T) {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	assert.Nil(t, NewCmdStack(cmdtest.NewTestFactory(nil)).Execute())

	out := test.ReturnBuffer(old, r, w)

	assert.Contains(t, out, "Stacked diffs are a way of creating small changes that build upon each other to ultimately deliver")
	assert.Contains(t, out, text.ExperimentalString)
}

func TestStackCmd_PullFlag(t *testing.T) {
	cmd := NewCmdStack(cmdtest.NewTestFactory(nil))

	// Verify the --pull flag exists on the stack command
	pullFlag := cmd.Flags().Lookup("pull")
	assert.NotNil(t, pullFlag, "--pull flag should exist on stack command")
	assert.Equal(t, "false", pullFlag.DefValue)
	assert.Contains(t, pullFlag.Usage, "Only pull and rebase changes")

	// Verify the sync subcommand also has the --pull flag
	syncCmd, _, err := cmd.Find([]string{"sync"})
	assert.NoError(t, err)
	assert.NotNil(t, syncCmd)

	syncPullFlag := syncCmd.Flags().Lookup("pull")
	assert.NotNil(t, syncPullFlag, "--pull flag should exist on sync subcommand")
	assert.Equal(t, "false", syncPullFlag.DefValue)
}
