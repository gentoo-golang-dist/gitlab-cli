package changelog

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	changelogGenerateCmd "gitlab.com/gitlab-org/cli/internal/commands/changelog/generate"
)

func NewCmdChangelog(f cmdutils.Factory) *cobra.Command {
	changelogCmd := &cobra.Command{
		Use:   "changelog <command> [flags]",
		Short: `Generate changelogs from your project's commit history.`,
		Long: heredoc.Doc(`
		Generate changelogs from commits in your project's Git repository.
		Changelog entries are built from commits that include a configured Git
		trailer, such as 'Changelog'.

		For more information about how GitLab generates changelogs, see
		[Changelogs](https://docs.gitlab.com/user/project/changelogs/).
		`),
	}

	// Subcommands
	changelogCmd.AddCommand(changelogGenerateCmd.NewCmdGenerate(f))

	return changelogCmd
}
