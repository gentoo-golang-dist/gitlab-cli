package token

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token/list"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <command> [flags]",
		Short: `Manage runner controller tokens. (EXPERIMENTAL)`,
		Long:  `Manages GitLab runner controller tokens. This is an admin-only feature.` + "\n" + text.ExperimentalString,
	}

	cmd.AddCommand(listCmd.NewCmd(f))
	return cmd
}
