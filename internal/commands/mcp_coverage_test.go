//go:build !integration

package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

// TestEveryLeafCommandHasMCPAnnotation fails when any leaf with a
// RunE lacks an MCP annotation. Missing annotations silently drop
// the command from the MCP tool surface; this catches that drift.
// It checks presence, not correctness of choice.
func TestEveryLeafCommandHasMCPAnnotation(t *testing.T) {
	factory := cmdutils.NewFactory(
		setupIOStreams(),
		false,
		config.NewBlankConfig(),
		api.BuildInfo{Version: "v1.0.0", Commit: "abcdefgh"},
	)
	root := NewCmdRoot(factory)

	var missing []string
	walkCommandTree(root, []string{}, func(cmd *cobra.Command, path []string) {
		if !isLeafCommand(cmd) {
			return
		}
		if !mcpannotations.HasAnnotation(cmd.Annotations) {
			missing = append(missing, "glab "+strings.Join(path, " "))
		}
	})

	assert.Empty(t, missing,
		"every leaf command must declare an MCP annotation (Safe, Destructive, "+
			"Interactive, or Exclude). Missing:\n  - %s",
		strings.Join(missing, "\n  - "),
	)
}

// isLeafCommand reports whether cmd runs something directly and
// has no non-hidden subcommands of its own.
func isLeafCommand(cmd *cobra.Command) bool {
	if cmd.RunE == nil && cmd.Run == nil {
		return false
	}
	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}
		return false
	}
	return true
}

// walkCommandTree visits every command below the root. Path excludes
// the binary name.
func walkCommandTree(cmd *cobra.Command, path []string, visit func(*cobra.Command, []string)) {
	if cmd.HasParent() {
		name := strings.Fields(cmd.Use)[0]
		path = append(path, name)
		visit(cmd, path)
	}
	for _, sub := range cmd.Commands() {
		walkCommandTree(sub, path, visit)
	}
}
