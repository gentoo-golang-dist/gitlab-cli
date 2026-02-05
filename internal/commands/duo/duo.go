package duo

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	duoAskCmd "gitlab.com/gitlab-org/cli/internal/commands/duo/ask"
)

func NewCmdDuo(f cmdutils.Factory) *cobra.Command {
	duoCmd := &cobra.Command{
		Use:   "duo <command> prompt",
		Short: "Work with GitLab Duo",
		Long: heredoc.Doc(`
			Work with GitLab Duo, our AI-native assistant for the command line.

			GitLab Duo for the CLI integrates AI capabilities directly into your terminal
			workflow. It helps you retrieve forgotten Git commands and offers guidance on
			Git operations. You can accomplish specific tasks without switching contexts.

			To interact with the GitLab Duo Agent Platform, use the
			[GitLab Duo CLI](https://docs.gitlab.com/user/gitlab_duo_cli/).

			A unified experience is proposed in
			[issue 585937](https://gitlab.com/gitlab-org/gitlab/-/work_items/585937).
		`),
	}

	duoCmd.AddCommand(duoAskCmd.NewCmdAsk(f))

	return duoCmd
}
