package agent

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	agentFileCmd "gitlab.com/gitlab-org/cli/internal/commands/agent/file"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdAgent(f cmdutils.Factory) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent <command> [flags]",
		Short: "API-backed primitives for agentic and CI workflows. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Read repository state without a local clone.

			Designed for agentic coding tools (Claude Code, Cursor, Copilot, etc.)
			and CI tooling that needs to answer common repository questions over
			the GitLab API instead of falling back to git clone.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			glab agent file get README.md --ref main`),
	}

	agentCmd.AddCommand(agentFileCmd.NewCmdFile(f))

	return agentCmd
}
