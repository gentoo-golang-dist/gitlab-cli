package remote

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	remoteCmdAdd "gitlab.com/gitlab-org/cli/internal/commands/project/remote/add"
)

func NewCmdRemote(f cmdutils.Factory) *cobra.Command {
	remoteCmd := &cobra.Command{
		Use:   "remote <subcommand>",
		Short: "Manage git remotes for a GitLab project.",
		Long:  ``,
	}

	remoteCmd.AddCommand(remoteCmdAdd.NewCmdRemoteAdd(f))

	return remoteCmd
}
