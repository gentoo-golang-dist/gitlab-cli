package orbit

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func newCleanCmd(f cmdutils.Factory) *cobra.Command {
	return &cobra.Command{
		Use:                "clean",
		Short:              "Remove all indexed data",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGkg(cmd.Context(), f.IO(), append([]string{"clean"}, args...))
		},
	}
}
