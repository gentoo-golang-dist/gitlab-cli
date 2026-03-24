package duo

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	duoAskCmd "gitlab.com/gitlab-org/cli/internal/commands/duo/ask"
	duoCLICmd "gitlab.com/gitlab-org/cli/internal/commands/duo/cli"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	// Create cli command first
	cliCmd := duoCLICmd.NewCmd(f)

	duoCmd := &cobra.Command{
		Use:   "duo <command> prompt",
		Short: "Work with GitLab Duo",
		Long: heredoc.Doc(`
			Work with GitLab Duo, our AI-native assistant for the command line.

			The GitLab Duo CLI integrates AI capabilities directly into your terminal
			workflow. It helps you retrieve forgotten Git commands and offers guidance on
			Git operations. You can accomplish specific tasks without switching contexts.

			To interact with the GitLab Duo Agent Platform, use the
			[GitLab Duo CLI](https://docs.gitlab.com/user/gitlab_duo_cli/).

			A unified experience is proposed in
			[epic 20826](https://gitlab.com/groups/gitlab-org/-/work_items/20826).
		`),
		// Default to running cli when no subcommand is provided
		RunE: cliCmd.RunE,
	}

	duoCmd.AddCommand(duoAskCmd.NewCmdAsk(f))
	duoCmd.AddCommand(cliCmd)

	// Allow unknown flags to be passed through to the Duo CLI binary
	duoCmd.FParseErrWhitelist.UnknownFlags = true

	return duoCmd
}
