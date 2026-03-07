package deploytokencmd

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/create"
	deleteCmd "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/delete"
	getCmd "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/get"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/list"
)

func NewCmdDeployToken(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-token",
		Short: "Manage deploy tokens for a project or group.",
	}

	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(getCmd.NewCmdGet(f))
	cmd.AddCommand(createCmd.NewCmdCreate(f))
	cmd.AddCommand(deleteCmd.NewCmdDelete(f))

	return cmd
}
