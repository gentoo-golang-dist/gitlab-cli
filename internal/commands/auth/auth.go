package auth

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	credentialHelperCmd "gitlab.com/gitlab-org/cli/internal/commands/auth/credentialhelper"
	authDockerCredentialHelperCmd "gitlab.com/gitlab-org/cli/internal/commands/auth/docker"
	cmdGenerate "gitlab.com/gitlab-org/cli/internal/commands/auth/generate"
	authLoginCmd "gitlab.com/gitlab-org/cli/internal/commands/auth/login"
	authLogoutCmd "gitlab.com/gitlab-org/cli/internal/commands/auth/logout"
	authStatusCmd "gitlab.com/gitlab-org/cli/internal/commands/auth/status"
)

func NewCmdAuth(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <command>",
		Short: "Manage authentication for glab.",
		Long: heredoc.Doc(`
		Manages authentication for glab against one or more GitLab instances. Use
		these commands to log in, log out, check your authentication status,
		and configure glab as a credential helper for Git and Docker.

		glab can authenticate with a personal access token, an OAuth token from
		a web flow, or a CI job token when running in a GitLab CI/CD job.
		`),
	}

	cmd.AddCommand(authLoginCmd.NewCmdLogin(f))
	cmd.AddCommand(authStatusCmd.NewCmdStatus(f, nil))
	cmd.AddCommand(authLoginCmd.NewCmdCredential(f))
	cmd.AddCommand(cmdGenerate.NewCmdGenerate(f))
	cmd.AddCommand(authLogoutCmd.NewCmdLogout(f))
	cmd.AddCommand(authDockerCredentialHelperCmd.NewCmdConfigureDocker(f))
	cmd.AddCommand(authDockerCredentialHelperCmd.NewCmdCredentialHelper(f))
	cmd.AddCommand(credentialHelperCmd.NewCmd(f))

	return cmd
}
