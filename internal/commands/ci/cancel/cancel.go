package cancel

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	cancelJobCmd "gitlab.com/gitlab-org/cli/internal/commands/ci/cancel/job"
	cancelPipelineCmd "gitlab.com/gitlab-org/cli/internal/commands/ci/cancel/pipeline"
)

func NewCmdCancel(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <command>",
		Short: "Cancel a running pipeline or job.",
		Long: heredoc.Docf(`
		Cancel one or more running CI/CD pipelines or jobs.

		Use the %[1]spipeline%[1]s subcommand to cancel pipelines, or the %[1]sjob%[1]s
		subcommand to cancel individual jobs.
		`, "`"),
	}

	cmd.AddCommand(cancelPipelineCmd.NewCmdCancel(f))
	cmd.AddCommand(cancelJobCmd.NewCmdCancel(f))

	return cmd
}
