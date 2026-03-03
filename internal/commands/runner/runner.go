package runner

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	assignCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/assign"
	deleteCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/delete"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/list"
	unassignCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/unassign"
	updateCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/update"
)

func NewCmdRunner(f cmdutils.Factory) *cobra.Command {
	runnerCmd := &cobra.Command{
		Use:   "runner <command> [flags]",
		Short: "Manage GitLab CI/CD runners.",
		Long:  "Manages GitLab CI/CD runners for projects, groups, or the entire instance.\n",
	}

	cmdutils.EnableRepoOverride(runnerCmd, f)

	runnerCmd.AddCommand(assignCmd.NewCmd(f))
	runnerCmd.AddCommand(listCmd.NewCmd(f))
	runnerCmd.AddCommand(updateCmd.NewCmd(f))
	runnerCmd.AddCommand(deleteCmd.NewCmd(f))
	runnerCmd.AddCommand(unassignCmd.NewCmd(f))

	return runnerCmd
}
