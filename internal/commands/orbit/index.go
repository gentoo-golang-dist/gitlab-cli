package orbit

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func newIndexCmd(f cmdutils.Factory) *cobra.Command {
	return &cobra.Command{
		Use:                "index [flags] [workspace-path]",
		Short:              "Index repositories in a workspace",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGkg(cmd.Context(), f.IO(), append([]string{"index"}, args...))
		},
	}
}
