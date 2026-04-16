//go:build !integration

package board

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdBoard(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdBoard(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Work with GitLab issue boards in the given project.\n")
}
