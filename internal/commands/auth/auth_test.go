//go:build !integration

package auth

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestIssueCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdAuth(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Manages authentication for glab against one or more GitLab instances.")
}
