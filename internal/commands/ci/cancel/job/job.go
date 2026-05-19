package job

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

const (
	FlagDryRun = "dry-run"
)

func NewCmdCancel(f cmdutils.Factory) *cobra.Command {
	jobCancelCmd := &cobra.Command{
		Use:   "job <id> [<id>...] [flags]",
		Short: `Cancel CI/CD jobs.`,
		Long: heredoc.Docf(`
		Cancels one or more running CI/CD jobs by ID. You can pass multiple
		job IDs as separate arguments, in a comma-separated list, or in a
		quoted space-separated list.

		To preview which jobs would be canceled without making changes, use
		%[1]s--dry-run%[1]s.
		`, "`"),
		Example: heredoc.Doc(`
			# Cancel a single job
			glab ci cancel job 1504182795

			# Cancel multiple jobs, comma-separated
			glab ci cancel job 1504182795,1504182796

			# Cancel multiple jobs, space-separated in quotes
			glab ci cancel job "1504182795 1504182796"

			# Preview which jobs would be canceled
			glab ci cancel job 1504182795,1504182796 --dry-run
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("You must pass a job ID.")
			}

			return nil
		},
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			c := f.IO().Color()
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			dryRunMode, _ := cmd.Flags().GetBool(FlagDryRun)

			var jobIDs []int

			jobIDs, err = ciutils.IDsFromArgs(args)
			if err != nil {
				return err
			}
			return runCancelation(jobIDs, dryRunMode, f.IO().StdOut, c, client, repo)
		},
	}

	SetupCommandFlags(jobCancelCmd.Flags())
	return jobCancelCmd
}

func SetupCommandFlags(flags *pflag.FlagSet) {
	flags.BoolP(FlagDryRun, "", false, "Show which jobs would be canceled, without canceling them.")
}

func runCancelation(
	jobIDs []int,
	dryRunMode bool,
	w io.Writer,
	c *iostreams.ColorPalette,
	apiClient *gitlab.Client,
	repo glrepo.Interface,
) error {
	for _, id := range jobIDs {
		if dryRunMode {
			fmt.Fprintf(w, "%s Job #%d will be canceled.\n", c.DotWarnIcon(), id)
		} else {
			pid, err := repo.Project(apiClient)
			if err != nil {
				return err
			}
			_, _, err = apiClient.Jobs.CancelJob(pid.ID, int64(id))
			if err != nil {
				return err
			}

			fmt.Fprintf(w, "%s Job #%d is canceled successfully.\n", c.RedCheck(), id)
		}
	}
	fmt.Println()

	return nil
}
