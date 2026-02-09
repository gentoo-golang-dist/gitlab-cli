package token

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token/create"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token/list"
	revokeCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token/revoke"
	rotateCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token/rotate"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <command> [flags]",
		Short: `Manage runner controller tokens. (EXPERIMENTAL)`,
		Long:  `Manages GitLab runner controller tokens. This is an admin-only feature.` + "\n" + text.ExperimentalString,
	}

	cmd.AddCommand(createCmd.NewCmd(f))
	cmd.AddCommand(listCmd.NewCmd(f))
	cmd.AddCommand(revokeCmd.NewCmd(f))
	cmd.AddCommand(rotateCmd.NewCmd(f))
	return cmd
}
