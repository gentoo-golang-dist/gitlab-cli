package deploytoken

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	cmdCreate "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/create"
	cmdDelete "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/delete"
	cmdGet "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/get"
	cmdList "gitlab.com/gitlab-org/cli/internal/commands/deploy-token/list"
)

func NewCmdDeployToken(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-token <command>",
		Short: "Manage deploy tokens.",
		Long:  "Manage deploy tokens for projects and groups.\n",
	}

	cmdutils.EnableRepoOverride(cmd, f)

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdGet.NewCmdGet(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))

	return cmd
}
