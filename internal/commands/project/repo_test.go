//go:build !integration

package project

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_Repo(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdRepo(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"repo [command] --help\" for more information about a command.\n")
}
