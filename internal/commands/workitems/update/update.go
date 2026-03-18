package update

import (
	"context"
	"fmt"
	"strings"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

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
	io *iostreams.IOStreams
	baseRepo func() (glrepo.Interface, error)
	gitlabClient func() (*gitlab.Client, error)
	config func() config.Config

	// Flags
	group string
	iid int64
	title string
	// TODO: ask about stateEvent
	// stateEvent string
	description string
	assigneeIDs []int64
	milestoneID int64
	parentID int64
	addLabelIDs []int64
	removeLabelIDs []int64
	startDate string
	dueDate string
	weight int64
	healthStatus string
	iterationID int64
	color string
	status string

	// internal state
	scope *api.ScopeInfo

	// Output
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io: f.IO(),
		baseRepo: f.BaseRepo,
		gitlabClient: f.GitLabClient,
		config: f.Config,
	}

	cmd := &cobra.Command{
		Use: "update <iid> [flags]",
		Short: "Update work items in a project or group. (EXPERIMENTAL)",
		Long: heredoc.Doc(`Update work items in a project or group.

		The command uses your repository context to detect scope automatically.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
					# Update work item in current project
					$ glab work-items update 42 --description "test description update"
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
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	// enable -R flag for repo override
	cmdutils.EnableRepoOverride(cmd, f)

	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	// Flags
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Update work items for a group or subgroup")
	cmd.Flags().StringVarP(&opts.title, "title", "t", "", "Update title for work item")
	cmd.Flags().StringVarP(&opts.description, "description", "d", "", "Update description for work item")

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
