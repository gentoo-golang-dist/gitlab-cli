package duo

import (
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	duoAskCmd "gitlab.com/gitlab-org/cli/commands/duo/ask"
	duoChatCmd "gitlab.com/gitlab-org/cli/commands/duo/chat"

	"github.com/spf13/cobra"
)

func NewCmdDuo(f *cmdutils.Factory) *cobra.Command {
	duoCmd := &cobra.Command{
		Use:   "duo <command> prompt",
		Short: "Generate terminal commands from natural language.",
		Long:  ``,
	}

	duoCmd.AddCommand(duoAskCmd.NewCmdAsk(f))
	duoCmd.AddCommand(duoChatCmd.NewCmdChat(f))

	return duoCmd
}
