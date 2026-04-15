package search

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	searchIssuesCmd "gitlab.com/gitlab-org/cli/internal/commands/search/issues"
)

// NewCmd creates the top-level `glab search` command.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	searchCmd := &cobra.Command{
		Use:   "search <command> [flags]",
		Short: "Search for GitLab resources.",
		Long: heredoc.Doc(`Search for GitLab resources across the instance, a group, or a project.

Scope is resolved automatically from context (git remote, -g, or -R flags).
`),
		Annotations: map[string]string{
			"help:group": "search",
		},
	}

	searchCmd.AddCommand(searchIssuesCmd.NewCmd(f))

	return searchCmd
}
