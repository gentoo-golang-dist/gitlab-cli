package compile

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	lintCmd "gitlab.com/gitlab-org/cli/internal/commands/ci/lint"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

func NewCmdConfigCompile(f cmdutils.Factory) *cobra.Command {
	configCompileCmd := &cobra.Command{
		Use:        "compile",
		Short:      "View the fully expanded CI/CD configuration.",
		Args:       cobra.MaximumNArgs(1),
		Deprecated: "use 'glab ci lint --include-merged-yaml' instead.",
		Example: heredoc.Doc(`
			# Uses .gitlab-ci.yml in the current directory
			glab ci config compile
			glab ci config compile .gitlab-ci.yml
			glab ci config compile path/to/.gitlab-ci.yml`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ".gitlab-ci.yml"
			if len(args) == 1 {
				path = args[0]
			}
			return compileRun(f, path)
		},
	}

	configCompileCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide "repo"-flag for this command, because it cannot be used on repositories but only on gitlab-ci files
		_ = configCompileCmd.Flags().MarkHidden("repo")

		configCompileCmd.Parent().HelpFunc()(command, strings)
	})

	return configCompileCmd
}

func compileRun(f cmdutils.Factory, path string) error {
	return lintCmd.RunMergedYAML(f, path)
}
