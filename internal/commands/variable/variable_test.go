//go:build !integration

package variable

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewVariableCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewVariableCmd(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"variable [command] --help\" for more information about a command.\n")
}
