package skills

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	installCmd "gitlab.com/gitlab-org/cli/internal/commands/skills/install"
	listCmd "gitlab.com/gitlab-org/cli/internal/commands/skills/list"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdSkills(f cmdutils.Factory) *cobra.Command {
	skillsCmd := &cobra.Command{
		Use:   "skills <command>",
		Short: "Manage glab agent skills. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Install the bundled glab agent skills so that AI agents can discover
			and use glab effectively.

			Skills follow the [Agent Skills specification](https://agentskills.io) and work with
			any compatible agent, including GitLab Duo Agent Platform, Claude Code, Codex,
			and Gemini CLI.
		`) + text.ExperimentalString,
	}

	skillsCmd.AddCommand(installCmd.NewCmdInstall(f))
	skillsCmd.AddCommand(listCmd.NewCmdList(f))

	return skillsCmd
}
