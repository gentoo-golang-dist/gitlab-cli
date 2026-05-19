package config

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	ConfigCompileCmd "gitlab.com/gitlab-org/cli/internal/commands/ci/config/compile"
)

func NewCmdConfig(f cmdutils.Factory) *cobra.Command {
	ConfigCmd := &cobra.Command{
		Use:   "config <command> [flags]",
		Short: `View and inspect GitLab CI/CD configuration.`,
		Long: heredoc.Docf(`
		View and inspect the CI/CD configuration for your GitLab project.

		Use the %[1]scompile%[1]s subcommand to view the fully merged
		%[1]s.gitlab-ci.yml%[1]s file, including all %[1]sinclude%[1]s directives resolved
		to their final form.
		`, "`"),
	}
	ConfigCmd.AddCommand(ConfigCompileCmd.NewCmdConfigCompile(f))
	return ConfigCmd
}
