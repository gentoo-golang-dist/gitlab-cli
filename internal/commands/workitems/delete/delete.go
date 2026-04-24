package delete

import (
	"fmt"
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
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)
	gitlabClient func() (*gitlab.Client, error)
	config       func() config.Config

	// Flags
	group string
	iid   int64

	// Internal state
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
		Use:   "delete <iid>",
		Short: "Delete a given work item in a project or group. (EXPERIMENTAL)",
		Long: heredoc.Doc(`The command uses your repository context to detect scope automatically.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
		# Delete work item in current project
		glab work-items delete 42

		# Delete work item in specific group
		glab work-items delete 42 --group MYGROUP
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
			if err := opts.complete(cmd); err != nil {
				return err
			}

			return opts.run()
		},
	}

	// enable -R flag fo repo override
	cmdutils.EnableRepoOverride(cmd, f)

	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	// Flags
	fl := cmd.Flags()
	fl.StringVarP(&opts.group, "group", "g", "", "Delete work items for a group or subgroup.")

	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (opts *options) complete(cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	opts.group = group

	scope, err := utils.DetectScope(opts.group, opts.baseRepo)
	if err != nil {
		return err
	}
	opts.scope = scope
	return nil
}

func (opts *options) run() error {
	client, err := opts.gitlabClient()
	if err != nil {
		return fmt.Errorf("failed to get GitLab client: %w", err)
	}

	fmt.Fprintln(opts.io.StdOut, "- Deleting work item in", opts.scope.Path)

	_, err = client.WorkItems.DeleteWorkItem(opts.scope.Path, opts.iid)
	if err != nil {
		return err
	}

	if opts.io.IsaTTY {
		fmt.Fprintf(opts.io.StdOut, "Successfully deleted %d\n", opts.iid)
	} else {
		fmt.Fprintln(opts.io.StdOut, opts.iid)
	}

	return nil
}
