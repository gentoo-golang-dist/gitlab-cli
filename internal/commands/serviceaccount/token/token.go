package token

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	createCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/token/create"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/token/list"
	revokeCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/token/revoke"
	rotateCmd "gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/token/rotate"
)

func NewCmdToken(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <command> [flags]",
		Short: "Manage personal access tokens for service accounts.",
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.AddCommand(createCmd.NewCmdCreate(f))
	cmd.AddCommand(listCmd.NewCmdList(f))
	cmd.AddCommand(revokeCmd.NewCmdRevoke(f))
	cmd.AddCommand(rotateCmd.NewCmdRotate(f))
	return cmd
}
