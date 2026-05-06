//go:build !integration

package label

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdLabel(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdLabel(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"label [command] --help\" for more information about a command.\n")
}
