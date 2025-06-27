package agent

import (
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/commands/duo/agent/chat"
)

func NewCmdAgent(f cmdutils.Factory) *cobra.Command {
	duoAgentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Work with agents",
		Long:  ``,
	}

	duoAgentCmd.AddCommand(chat.NewCmdChat(f))

	return duoAgentCmd
}
