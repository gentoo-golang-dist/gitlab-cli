package orbit

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	localCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/local"
	remoteCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// NewCmd returns the parent `glab orbit` command. It is a thin
// router that registers sub-trees for the remote API today and is the
// future home of local Knowledge Graph operations.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	orbitCmd := &cobra.Command{
		Use:   "orbit <command> [flags]",
		Short: `GitLab Knowledge Graph commands. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Access the GitLab Knowledge Graph (product name: Orbit) from the
			CLI. Use `+"`glab orbit remote`"+` to query the remote API, or
			`+"`glab orbit local`"+` to run the Orbit local CLI binary.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Discover the remote Knowledge Graph
			$ glab orbit remote status
			$ glab orbit remote schema
			$ glab orbit remote tools

			# Run a query against the remote Knowledge Graph
			$ glab orbit remote query ./query.json

			# Inspect indexing progress for a namespace or project
			$ glab orbit remote graph-status --full-path gitlab-org/gitlab

			# Run the Orbit local CLI (downloads the binary on first use)
			$ glab orbit local
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
	}

	orbitCmd.AddCommand(remoteCmd.NewCmd(f))
	orbitCmd.AddCommand(localCmd.NewCmd(f))

	return orbitCmd
}
