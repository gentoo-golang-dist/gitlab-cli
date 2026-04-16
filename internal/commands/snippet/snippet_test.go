//go:build !integration

package snippet

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCmdSnippet_noArgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdSnippet(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Use \"snippet [command] --help\" for more information about a command.\n")
}
