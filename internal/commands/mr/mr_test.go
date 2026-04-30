//go:build !integration

package mr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "mr_cmd_test")
	cmdtest.InitTest(m, "mr_cmd_autofill")
}

func TestMrCmd_noARgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdMR(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"mr [command] --help\" for more information about a command.\n")
}
