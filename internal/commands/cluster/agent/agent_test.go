//go:build !integration

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdAgent(t *testing.T) {
	ios, _, stdout, _ := cmdtest.TestIOStreams()
	cmd := NewCmdAgent(cmdtest.NewTestFactory(ios))
	cmd.SetOut(stdout)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "Register new agents, configure existing ones")
}
