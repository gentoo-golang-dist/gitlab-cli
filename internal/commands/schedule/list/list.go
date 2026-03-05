package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

var getSchedules = func(client *gitlab.Client, l *gitlab.ListPipelineSchedulesOptions, repo string) ([]*gitlab.PipelineSchedule, error) {
	schedules, _, err := client.PipelineSchedules.ListPipelineSchedules(repo, l)
	return schedules, err
}

type options struct {
	outputFormat string

	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}

	scheduleListCmd := &cobra.Command{
		Use:   "list [flags]",
		Short: `Get the list of schedules.`,
		Example: heredoc.Doc(`
			# List all scheduled pipelines
			$ glab schedule list
			> Showing schedules for project gitlab-org/cli
			> ID  Description                    Cron            Ref    Active
			> 1   Daily build                   0 0 * * *       main   true
			> 2   Weekly deployment             0 0 * * 0       main   true
		`),
		Long: ``,
		Args: cobra.ExactArgs(0),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.gitlabClient()
			if err != nil {
				return err
			}

			repo, err := opts.baseRepo()
			if err != nil {
				return err
			}

			l := &gitlab.ListPipelineSchedulesOptions{}
			page, _ := cmd.Flags().GetInt("page")
			l.Page = int64(page)
			perPage, _ := cmd.Flags().GetInt("per-page")
			l.PerPage = int64(perPage)

			schedules, err := getSchedules(client, l, repo.FullName())
			if err != nil {
				return err
			}

			if opts.outputFormat == "json" {
				return opts.io.PrintJSON(schedules)
			}

			title := utils.NewListTitle("schedule")
			title.RepoName = repo.FullName()
			title.Page = int(l.Page)
			title.CurrentPageTotal = len(schedules)

			fmt.Fprintf(opts.io.StdOut, "%s\n%s\n", title.Describe(), ciutils.DisplaySchedules(opts.io, schedules, repo.FullName()))
			return nil
		},
	}
	scheduleListCmd.Flags().IntP("page", "p", 1, "Page number.")
	scheduleListCmd.Flags().IntP("per-page", "P", 30, "Number of items to list per page.")
	cmdutils.EnableJSONOutput(scheduleListCmd, &opts.outputFormat)

	return scheduleListCmd
}
