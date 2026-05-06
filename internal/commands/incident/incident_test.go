//go:build !integration

package incident

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestIncidentCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdIncident(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Work with GitLab incidents.\n")
}
