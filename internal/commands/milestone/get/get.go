package get

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	projectID    string
	groupID      string
	milestoneID  int64
	outputFormat string
}

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:   "get [<id>] [flags]",
		Short: "Get a milestone by ID in a project or group.",
		Long: heredoc.Docf(`
		Get information about a single milestone in a project or group,
		identified by its numeric ID. Use %[1]s--output json%[1]s to format the result
		as JSON for use with other tools.

		By default, the milestone is looked up in the current project. Use
		%[1]s--project%[1]s to target a different project, or %[1]s--group%[1]s to look up a
		group-level milestone. %[1]s--project%[1]s and %[1]s--group%[1]s are mutually exclusive.
		`, "`"),
		Example: heredoc.Doc(`
			# Get a milestone from the current project
			glab milestone get 123

			# Get a milestone from a different project
			glab milestone get 123 --project owner/project

			# Get a group milestone
			glab milestone get 123 --group example-group

			# Get a milestone as JSON
			glab milestone get 123 --output json
		`),
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				milestoneIDInt, err := strconv.Atoi(args[0])
				if err != nil {
					return err
				}
				opts.milestoneID = int64(milestoneIDInt)
			}

			return opts.run()
		},
	}

	cmd.Flags().StringVar(&opts.projectID, "project", "", "The ID or URL-encoded path of the project.")
	cmd.Flags().StringVar(&opts.groupID, "group", "", "The ID or URL-encoded path of the group.")
	cmdutils.EnableJSONOutput(cmd, opts.io, &opts.outputFormat)

	return cmd
}

func (o *options) run() error {
	c, err := o.apiClient("")
	if err != nil {
		return err
	}
	client := c.Lab()

	if o.projectID != "" { // get project milestone
		milestone, _, err := client.Milestones.GetMilestone(o.projectID, o.milestoneID)
		if err != nil {
			return err
		}

		if o.outputFormat == "json" {
			return o.io.PrintJSON(milestone)
		}

		o.io.LogInfo(fmt.Sprintf("Title: %s\nDescription: %s\nState: %s\nDue Date: %s\n", milestone.Title, milestone.Description, milestone.State, utils.FormatDueDate(milestone.DueDate)))
		return nil
	} else if o.groupID != "" { // get group milestone
		milestone, _, err := client.GroupMilestones.GetGroupMilestone(o.groupID, o.milestoneID)
		if err != nil {
			return err
		}

		if o.outputFormat == "json" {
			return o.io.PrintJSON(milestone)
		}

		o.io.LogInfo(fmt.Sprintf("Title: %s\nDescription: %s\nState: %s\nDue Date: %s\n", milestone.Title, milestone.Description, milestone.State, utils.FormatDueDate(milestone.DueDate)))
		return nil
	}

	// run for the current project
	repo, _ := o.baseRepo()
	milestone, _, err := client.Milestones.GetMilestone(repo.FullName(), o.milestoneID)
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(milestone)
	}

	o.io.LogInfo(fmt.Sprintf("Title: %s\nDescription: %s\nState: %s\nDue Date: %s\n", milestone.Title, milestone.Description, milestone.State, utils.FormatDueDate(milestone.DueDate)))
	return nil
}
