package trace

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

func NewCmdTrace(f cmdutils.Factory) *cobra.Command {
	pipelineCITraceCmd := &cobra.Command{
		Use:   "trace [<job-id>|<job-name>] [flags]",
		Short: `Trace a CI/CD job log in real time.`,
		Example: heredoc.Doc(`
			# Interactively select a job to trace
			$ glab ci trace

			# Trace job with ID 224356863
			$ glab ci trace 224356863

			# Trace job with the name 'lint'
			$ glab ci trace lint

			# Follow a running job's log output (like tail -f)
			$ glab ci trace lint -f

			# Follow a job by ID
			$ glab ci trace 224356863 --follow

			# Output job log formatted for LLM consumption
			$ glab ci trace lint --format llm

			# Clean output without ANSI codes
			$ glab ci trace lint --format clean
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}
			jobName := ""
			if len(args) != 0 {
				jobName = args[0]
			}
			branch, _ := cmd.Flags().GetString("branch")
			pipelineId, _ := cmd.Flags().GetInt("pipeline-id")
			follow, _ := cmd.Flags().GetBool("follow")
			format, _ := cmd.Flags().GetString("format")

			return ciutils.TraceJob(cmd.Context(), &ciutils.JobInputs{
				JobName:    jobName,
				Branch:     branch,
				PipelineId: pipelineId,
			}, &ciutils.JobOptions{
				Client:     client,
				IO:         f.IO(),
				Repo:       repo,
				BranchFunc: f.Branch,
				Follow:     follow,
				Format:     ciutils.LogFormat(format),
			})
		},
	}

	pipelineCITraceCmd.Flags().StringP("branch", "b", "", "The branch to search for the job. (default current branch)")
	pipelineCITraceCmd.Flags().IntP("pipeline-id", "p", 0, "The pipeline ID to search for the job.")
	pipelineCITraceCmd.Flags().BoolP("follow", "f", false, "Follow job log output as it runs, similar to 'tail -f'.")
	pipelineCITraceCmd.Flags().String("format", "raw", "Output format: raw (default), clean (no ANSI codes), llm (optimized for LLM consumption).")
	return pipelineCITraceCmd
}
