//go:build !integration

package user

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestIssueCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdUser(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Interact with a GitLab user account.\n")
}
