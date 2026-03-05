package runnercontroller

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/create"
	deleteCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/delete"
	getCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/get"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/list"
	scopeCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/scope"
	tokenCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/token"
	updateCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/update"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runner-controller <command> [flags]",
		Short:   `Manage runner controllers. (EXPERIMENTAL)`,
		Long:    `Manages runner controllers. This is an administrator-only feature.` + "\n" + text.ExperimentalString,
		Aliases: []string{"rc"},
	}

	cmd.AddCommand(createCmd.NewCmd(f))
	cmd.AddCommand(deleteCmd.NewCmd(f))
	cmd.AddCommand(getCmd.NewCmd(f))
	cmd.AddCommand(listCmd.NewCmd(f))
	cmd.AddCommand(scopeCmd.NewCmd(f))
	cmd.AddCommand(tokenCmd.NewCmd(f))
	cmd.AddCommand(updateCmd.NewCmd(f))
	return cmd
}
