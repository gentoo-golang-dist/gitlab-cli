package duo

import (
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	duoAskCmd "gitlab.com/gitlab-org/cli/internal/commands/duo/ask"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
)

func NewCmdDuo(f cmdutils.Factory) *cobra.Command {
	duoCmd := &cobra.Command{
		Use:   "duo <command> prompt",
		Short: "Work with GitLab Duo",
		Long: heredoc.Doc(`
			Work with GitLab Duo, GitLab's AI-powered assistant for the command line.

			GitLab Duo for the CLI integrates AI capabilities directly into your terminal workflow,
			helping you retrieve forgotten Git commands and offering guidance on executing Git operations
			to accomplish specific tasks. This eliminates the need to switch contexts while working.
		`),
	}

	duoCmd.AddCommand(duoAskCmd.NewCmdAsk(f))

	return duoCmd
}
