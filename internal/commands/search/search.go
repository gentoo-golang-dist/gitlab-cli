package search

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	semanticCmd "gitlab.com/gitlab-org/cli/internal/commands/search/semantic"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	searchCmd := &cobra.Command{
		Use:   "search <command> [flags]",
		Short: `Search for code and resources in a GitLab project. (BETA)`,
		Long:  ``,
	}

	cmdutils.EnableRepoOverride(searchCmd, f)

	searchCmd.AddCommand(semanticCmd.NewCmd(f))

	return searchCmd
}
