package scope

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/scope/create"
	deleteCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/scope/delete"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/scope/list"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope <command> [flags]",
		Short: `Manage runner controller scopes. (EXPERIMENTAL)`,
		Long:  `Manages runner controller scopes. This is an administrator-only feature.` + "\n" + text.ExperimentalString,
	}

	cmd.AddCommand(createCmd.NewCmd(f))
	cmd.AddCommand(deleteCmd.NewCmd(f))
	cmd.AddCommand(listCmd.NewCmd(f))
	return cmd
}
