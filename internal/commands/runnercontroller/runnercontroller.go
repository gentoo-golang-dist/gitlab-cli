package runnercontroller

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/runnercontroller/list"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runner-controller <command> [flags]",
		Short:   `Manage runner controllers. (EXPERIMENTAL)`,
		Long:    `Manages GitLab runner controllers. This is an admin-only feature.` + "\n" + text.ExperimentalString,
		Aliases: []string{"rc"},
	}

	cmd.AddCommand(listCmd.NewCmd(f))
	return cmd
}
