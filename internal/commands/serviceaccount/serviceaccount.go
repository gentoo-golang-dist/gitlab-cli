package serviceaccount

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/create"
	deleteCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/delete"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/list"
	tokenCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/token"
	updateCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/update"
)

func NewCmdServiceAccount(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-account <command> [flags]",
		Short: "Manage GitLab service accounts.",
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.AddCommand(createCmd.NewCmdCreate(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(updateCmd.NewCmdUpdate(f))
	cmd.AddCommand(deleteCmd.NewCmdDelete(f))
	cmd.AddCommand(tokenCmd.NewCmdToken(f))
	return cmd
}
