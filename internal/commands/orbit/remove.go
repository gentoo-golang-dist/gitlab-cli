package orbit

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func newRemoveCmd(f cmdutils.Factory) *cobra.Command {
	return &cobra.Command{
		Use:                "remove [flags]",
		Short:              "Remove a workspace or project from the knowledge graph",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGkg(cmd.Context(), f.IO(), append([]string{"remove"}, args...))
		},
	}
}
