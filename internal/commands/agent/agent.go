package agent

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	setupCmd "gitlab.com/gitlab-org/cli/internal/commands/agent/setup"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdAgent(f cmdutils.Factory) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent <command>",
		Short: "Set up glab for use with AI coding agents. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Configure glab for use with AI coding agents.

			Install glab's bundled agent skills so that AI agents can discover
			and use glab effectively. Skills follow the Agent Skills specification
			(https://agentskills.io) and work with any compatible agent, including
			GitLab Duo, Claude Code, Codex, Gemini CLI, and others.
		`) + text.ExperimentalString,
	}

	agentCmd.AddCommand(setupCmd.NewCmdSetup(f))

	return agentCmd
}
