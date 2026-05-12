//go:build !integration

package orbit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd_RegistersRemoteSubcommand(t *testing.T) {
	t.Parallel()
	// GIVEN a fresh factory
	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdtest.NewTestFactory(ios)

	// WHEN the parent orbit command is constructed
	cmd := NewCmd(f)

	// THEN it carries the `remote` and `local` subtrees and the mcp:safe
	// annotation; the individual API commands are registered under
	// `remote`, not directly under `orbit`.
	gotSubs := make(map[string]bool, 2)
	for _, sub := range cmd.Commands() {
		gotSubs[sub.Name()] = true
	}
	assert.True(t, gotSubs["remote"], "expected `remote` subcommand to be registered")
	assert.True(t, gotSubs["local"], "expected `local` subcommand to be registered")
	for _, gone := range []string{"status", "schema", "tools", "query"} {
		assert.Falsef(t, gotSubs[gone],
			"expected %q to no longer be registered directly on `orbit`", gone)
	}
	assert.Equal(t, "true", cmd.Annotations["mcp:safe"])
}
