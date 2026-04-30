//go:build !integration

package securefile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_Securefile(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdSecurefile(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"securefile [command] --help\" for more information about a command.\n")
}
