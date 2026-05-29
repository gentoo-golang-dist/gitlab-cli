package remote

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	remoteCmdAdd "gitlab.com/gitlab-org/cli/internal/commands/project/remote/add"
)

func NewCmdRemote(f cmdutils.Factory) *cobra.Command {
	remoteCmd := &cobra.Command{
		Use:   "remote <subcommand>",
		Short: "Manage Git remotes for a GitLab project.",
		Long: heredoc.Doc(`
			Manage Git remotes for GitLab projects using project references instead of full URLs.
		`),
	}

	remoteCmd.AddCommand(remoteCmdAdd.NewCmdRemoteAdd(f))

	return remoteCmd
}
