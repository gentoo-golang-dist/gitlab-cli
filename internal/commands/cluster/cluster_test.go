//go:build !integration

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdCluster(t *testing.T) {
	ios, _, stdout, _ := cmdtest.TestIOStreams()
	cmd := NewCmdCluster(cmdtest.NewTestFactory(ios))
	cmd.SetOut(stdout)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "Manage GitLab Agents for Kubernetes and their clusters")
}
