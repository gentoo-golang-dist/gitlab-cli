//go:build !integration

package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd_RegistersAllSubcommands(t *testing.T) {
	t.Parallel()
	// GIVEN a fresh factory
	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdtest.NewTestFactory(ios)

	// WHEN the parent remote command is constructed
	cmd := NewCmd(f)

	// THEN it carries every documented API subcommand and the mcp:safe annotation
	gotSubs := make(map[string]bool, 5)
	for _, sub := range cmd.Commands() {
		gotSubs[sub.Name()] = true
	}
	for _, want := range []string{"status", "schema", "tools", "query", "graph-status"} {
		assert.Truef(t, gotSubs[want], "expected subcommand %q to be registered", want)
	}
	assert.Equal(t, "true", cmd.Annotations["mcp:safe"])
	assert.Contains(t, cmd.Aliases, "r")
}
