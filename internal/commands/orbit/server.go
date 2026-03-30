package orbit

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func newServerCmd(f cmdutils.Factory) *cobra.Command {
	return &cobra.Command{
		Use:                "server [command] [flags]",
		Short:              "Manage the gkg server",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGkg(cmd.Context(), f.IO(), append([]string{"server"}, args...))
		},
	}
}
