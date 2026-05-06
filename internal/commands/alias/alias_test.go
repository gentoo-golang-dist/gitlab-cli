//go:build !integration

package alias

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_Alias(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdAlias(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"alias [command] --help\" for more information about a command.\n")
}
