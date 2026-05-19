package config

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	editCmd "gitlab.com/gitlab-org/cli/internal/commands/config/edit"
	getCmd "gitlab.com/gitlab-org/cli/internal/commands/config/get"
	setCmd "gitlab.com/gitlab-org/cli/internal/commands/config/set"
)

func NewCmdConfig(f cmdutils.Factory) *cobra.Command {
	var isGlobal bool

	configCmd := &cobra.Command{
		Use:   "config [flags]",
		Short: `Manage glab settings.`,
		Long: heredoc.Docf(`Manage key/value strings.

		Current respected settings:

		- %[1]sbrowser%[1]s: If unset, uses the default browser. Override with environment variable %[1]s$BROWSER%[1]s.
		- %[1]scheck_update%[1]s: If true, notifies of new versions of glab. Defaults to %[1]strue%[1]s. Override with environment variable %[1]s$GLAB_CHECK_UPDATE%[1]s.
		- %[1]sdisplay_hyperlinks%[1]s: If %[1]sfalse%[1]s, disables hyperlinks in terminal output. Defaults to %[1]strue%[1]s. Override with environment variable %[1]s$FORCE_HYPERLINKS%[1]s.
		- %[1]seditor%[1]s: If unset, uses the default editor. Override with environment variable %[1]s$EDITOR%[1]s.
		- %[1]sglab_pager%[1]s: Your desired pager command to use, such as %[1]sless -R%[1]s.
		- %[1]sglamour_style%[1]s: Your desired Markdown renderer style. Options are dark, light, notty. Custom styles are available using [glamour](https://github.com/charmbracelet/glamour#styles).
		- %[1]shost%[1]s: If unset, defaults to %[1]shttps://gitlab.com%[1]s.
		- %[1]stoken%[1]s: Your GitLab access token. Defaults to environment variables.
		- %[1]svisual%[1]s: Takes precedence over %[1]seditor%[1]s. If unset, uses the default editor. Override with environment variable %[1]s$VISUAL%[1]s.
		`, "`"),
		Aliases: []string{"conf"},
	}

	configCmd.Flags().BoolVarP(&isGlobal, "global", "g", false, "Use global config file.")

	configCmd.AddCommand(getCmd.NewCmdGet(f))
	configCmd.AddCommand(setCmd.NewCmdSet(f))
	configCmd.AddCommand(editCmd.NewCmdEdit(f))

	return configCmd
}
