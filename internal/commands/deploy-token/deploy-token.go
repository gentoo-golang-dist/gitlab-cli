package token

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/create"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/list"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/revoke"
)

func NewDeployTokenCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deploy-token",
		Short:   "Manage personal, project, or group deploy tokens",
		Aliases: []string{"deploy-token"},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.AddCommand(create.NewCmdCreate(f))
	cmd.AddCommand(revoke.NewCmdRevoke(f))
	cmd.AddCommand(list.NewCmdList(f))
	return cmd
}
