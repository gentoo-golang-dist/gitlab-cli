package update

import (
	"context"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	a "gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/utils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	// Dependencies
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)
	gitlabClient func() (*gitlab.Client, error)
	config       func() config.Config

	// Flags
	group        string
	iid          int64
	title        string
	description  string
	assignee     []string
	milestone    string
	startDate    string
	dueDate      string
	weight       int64
	healthStatus string
	color        string
	status       string

	// internal state
	scope *api.ScopeInfo

	// Output
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		baseRepo:     f.BaseRepo,
		gitlabClient: f.GitLabClient,
		config:       f.Config,
	}

	cmd := &cobra.Command{
		Use:   "update <iid> [flags]",
		Short: "Update work items in a project or group. (EXPERIMENTAL)",
		Long: heredoc.Doc(`The command uses your repository context to detect scope automatically.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
					# Update work item in current project
					glab work-items update 42 --description "test description update in issue"

					glab work-items update 40 --group MYGROUP --description "test description update in epic"
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			iid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid work item ID: %w", err)
			}
			opts.iid = iid
			if err := opts.complete(cmd.Context(), cmd); err != nil {
				return err
			}

			return opts.run(cmd)
		},
	}

	// enable -R flag for repo override
	cmdutils.EnableRepoOverride(cmd, f)

	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	// Flags
	fl := cmd.Flags()
	fl.StringVarP(&opts.group, "group", "g", "", "Update work items for a group or subgroup.")
	fl.StringVarP(&opts.title, "title", "t", "", "Update title for work item.")
	fl.StringVarP(&opts.description, "description", "d", "", "Update description for work item.")
	fl.Int64VarP(&opts.weight, "weight", "w", 0, "Update weight value for the work item.")
	fl.Var(cmdutils.NewEnumValue([]string{"on-track", "needs-attention", "at-risk"}, "", &opts.healthStatus), "health", "Update health status for the work item: on-track, needs-attention or at-risk.")
	fl.Var(cmdutils.NewEnumValue([]string{"to-do", "in-progress", "done", "wont-do", "duplicate"}, "", &opts.status), "status", "Update current status for the work item: to-do, in-progress, done, wont-do, duplicate.")
	fl.StringVarP(&opts.color, "color", "c", "", "Update the Color for the work item, as a CSS color string. Typically a hex code like #e24329; named colors are also accepted.")
	fl.StringSliceVarP(&opts.assignee, "assignee", "a", []string{}, "Update work item assignee with the supplied GitLab usernames.")
	fl.StringVarP(&opts.milestone, "milestone", "m", "", "Update work item milestone with the title or ID.")
	fl.StringVar(&opts.startDate, "startdate", "", "Update Start Date for a given work item.")
	fl.StringVar(&opts.dueDate, "duedate", "", "Update Due Date for a given work item.")

	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (opts *options) complete(ctx context.Context, cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	opts.group = group

	if err := cmdutils.HandleDescriptionEditor(ctx, &opts.description, opts.io, opts.config, nil); err != nil {
		return err
	}

	scope, err := utils.DetectScope(opts.group, opts.baseRepo)
	if err != nil {
		return err
	}
	opts.scope = scope

	return nil
}

func (opts *options) run(cmd *cobra.Command) error {
	client, err := opts.gitlabClient()
	if err != nil {
		return fmt.Errorf("failed to get GitLab client: %w", err)
	}

	updateOpts := gitlab.UpdateWorkItemOptions{}

	if opts.startDate != "" {
		startDate, err := gitlab.ParseISOTime(opts.startDate)
		if err != nil {
			return cmdutils.FlagError{Err: fmt.Errorf("date is not formatted correctly")}
		}
		updateOpts.StartDate = new(startDate)
	}

	if opts.dueDate != "" {
		dueDate, err := gitlab.ParseISOTime(opts.dueDate)
		if err != nil {
			return cmdutils.FlagError{Err: fmt.Errorf("date is not formatted correctly")}
		}
		updateOpts.DueDate = new(dueDate)
	}

	if opts.title != "" {
		updateOpts.Title = new(opts.title)
	}

	if opts.description != "" {
		updateOpts.Description = new(opts.description)
	}

	if len(opts.assignee) != 0 {
		user, err := a.UsersByNames(client, opts.assignee)
		if err != nil {
			return cmdutils.FlagError{Err: fmt.Errorf("failed to find assignee: %w", err)}
		}

		assignees := make([]int64, 0)

		for _, i := range user {
			assignees = append(assignees, i.ID)
		}
		updateOpts.AssigneeIDs = assignees
	}

	if opts.milestone != "" {
		if ok, err := strconv.ParseInt(opts.milestone, 10, 64); err == nil {
			updateOpts.MilestoneID = new(ok)
		} else {
			if opts.scope.Type == "project" {

				l := &a.ListMilestonesOptions{
					Title: new(opts.milestone),
				}

				m, err := a.ListAllMilestones(client, opts.scope.Path, l)
				if err != nil || len(m) == 0 {
					return cmdutils.FlagError{Err: fmt.Errorf("failed to find project milestone by title")}
				}

				updateOpts.MilestoneID = new(m[0].ID)
			} else if opts.scope.Type == "group" {
				m, _, err := client.GroupMilestones.ListGroupMilestones(opts.scope.Path, &gitlab.ListGroupMilestonesOptions{Title: new(opts.milestone)})
				if err != nil || len(m) == 0 {
					return cmdutils.FlagError{Err: fmt.Errorf("failed to find group milestone by title")}
				}

				updateOpts.MilestoneID = new(m[0].ID)
			}
		}
	}

	if cmd.Flags().Changed("weight") {
		updateOpts.Weight = new(opts.weight)
	}

	switch opts.healthStatus {
	case "on-track":
		updateOpts.HealthStatus = new("onTrack")
	case "needs-attention":
		updateOpts.HealthStatus = new("needsAttention")
	case "at-risk":
		updateOpts.HealthStatus = new("atRisk")
	}

	if opts.color != "" {
		updateOpts.Color = new(opts.color)
	}

	switch opts.status {
	case "to-do":
		updateOpts.Status = gitlab.Ptr(gitlab.WorkItemStatusToDo)
	case "in-progress":
		updateOpts.Status = gitlab.Ptr(gitlab.WorkItemStatusInProgress)
	case "done":
		updateOpts.Status = gitlab.Ptr(gitlab.WorkItemStatusDone)
	case "wont-do":
		updateOpts.Status = gitlab.Ptr(gitlab.WorkItemStatusWontDo)
	case "duplicate":
		updateOpts.Status = gitlab.Ptr(gitlab.WorkItemStatusDuplicate)
	default:
		// unreachable: validate() ensures only valid statuses reach here
	}

	wi, _, err := client.WorkItems.UpdateWorkItem(opts.scope.Path, opts.iid, &updateOpts)
	if err != nil {
		return err
	}

	switch opts.outputFormat {
	case "json":
		return opts.io.PrintJSON(wi)
	default:
		fmt.Fprintln(opts.io.StdOut, "- Updating work item in", opts.scope.Path)

		if opts.io.IsaTTY {
			fmt.Fprintf(opts.io.StdOut, "#%d %s\n%s\n", wi.IID, wi.Title, wi.WebURL)
		} else {
			fmt.Fprintln(opts.io.StdOut, wi.WebURL)
		}
	}

	return nil
}
