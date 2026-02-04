package skill

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	addCmd "gitlab.com/gitlab-org/cli/internal/commands/skill/add"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

//go:embed SKILL.md
var skillTemplate string

const dynamicCommandListPlaceholder = "<% DYNAMIC_COMMAND_LIST %>"

func NewCmdSkill(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill [command]",
		Short: "Manage the glab Agent Skill for AI coding agents",
		Long: heredoc.Doc(`
			Print or install the glab skill file for AI coding agents, based on the [Agent Skills](https://agentskills.io) specification.

			Running 'glab skill' with no arguments prints the skill content to stdout.
			Use 'glab skill add <path>' to write the skill to a file.
		`),
		Example: heredoc.Doc(`
			$ glab skill
			$ glab skill add ./skills/glab/SKILL.md
			$ glab skill add ~/.opencode/skills/glab/SKILL.md
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content := generateSkillContent(cmd.Root())
			fmt.Fprint(f.IO().StdOut, content)
			return nil
		},
	}

	cmd.AddCommand(addCmd.NewCmdAdd(f, func(root *cobra.Command) string {
		return generateSkillContent(root)
	}))

	return cmd
}

func generateSkillContent(root *cobra.Command) string {
	commandList := generateCommandList(root)
	return strings.Replace(skillTemplate, dynamicCommandListPlaceholder, commandList, 1)
}

func generateCommandList(root *cobra.Command) string {
	var buf bytes.Buffer

	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" || !cmd.IsAvailableCommand() {
			continue
		}

		buf.WriteString(fmt.Sprintf("- `glab %s` - %s\n", cmd.Name(), cmd.Short))

		if cmd.HasAvailableSubCommands() {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "help" || !sub.IsAvailableCommand() {
					continue
				}
				buf.WriteString(fmt.Sprintf("  - `glab %s %s` - %s\n", cmd.Name(), sub.Name(), sub.Short))
			}
		}
	}

	return buf.String()
}
