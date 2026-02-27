package runner

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/list"
	updateCmd "gitlab.com/gitlab-org/cli/internal/commands/runner/update"
)

func NewCmdRunner(f cmdutils.Factory) *cobra.Command {
	runnerCmd := &cobra.Command{
		Use:   "runner <command> [flags]",
		Short: "Manage GitLab CI/CD runners.",
		Long:  "Manages GitLab CI/CD runners for projects, groups, or the entire instance.\n",
	}

	cmdutils.EnableRepoOverride(runnerCmd, f)

	runnerCmd.AddCommand(listCmd.NewCmd(f))
	runnerCmd.AddCommand(updateCmd.NewCmd(f))

	return runnerCmd
}
