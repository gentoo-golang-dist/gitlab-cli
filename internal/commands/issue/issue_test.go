//go:build !integration

package issue

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestIssueCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdIssue(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Work with GitLab issues.\n")
}
