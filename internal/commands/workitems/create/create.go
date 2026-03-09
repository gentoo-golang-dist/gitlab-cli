package create

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
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
	title        string
	workItemType string
	description  string
	confidential bool

	// Internal state
	needsPrompt bool
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		baseRepo:     f.BaseRepo,
		gitlabClient: f.GitLabClient,
		config:       f.Config,
	}

	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Create work items in a project or group. (EXPERIMENTAL)",
		Long: heredoc.Doc(`Create work items in a project or group.

Automatically detects scope from repository context. 
Use --group flag for group-level work items or -R to specify a different project.
`) + text.ExperimentalString,
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
										# Create work item in current project
										glab work-items create --type issue
		`),
		Args: cobra.ExactArgs(0),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	// Enable -R flag for repo override
	cmdutils.EnableRepoOverride(cmd, f)

	// Flags
	cmd.Flags().StringP("group", "g", "", "Create work items for a group or subgroup")
	cmd.Flags().StringVarP(&opts.workItemType, "type", "T", "", "Type of work item ("+strings.Join(utils.ValidTypeNames(), ", ")+").")

	cmd.Flags().StringVarP(&opts.title, "title", "t", "", "Add title for work item")
	cmd.Flags().StringVarP(&opts.description, "description", "d", "", "Description of the work item. Set to \"-\" to open an editor.")
	cmd.Flags().BoolVarP(&opts.confidential, "confidential", "c", false, "Mark work item confidential.")

	return cmd
}

func (opts *options) complete(cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	opts.group = group
	opts.needsPrompt = !cmd.Flags().Changed("title")
	return nil
}

func (opts *options) validate() error {
	if opts.workItemType == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("--type is required")}
	}

	if _, err := utils.ResolveTypeID(opts.workItemType); err != nil {
		return err
	}

	if opts.needsPrompt && !opts.io.IsInteractive() {
		return cmdutils.FlagError{Err: fmt.Errorf("--title required for non-interactive mode")}
	}

	return nil
}

func (opts *options) run(ctx context.Context) error {
	scope, err := utils.DetectScope(opts.group, opts.baseRepo)
	if err != nil {
		return err
	}

	client, err := opts.gitlabClient()
	if err != nil {
		return fmt.Errorf("failed to get GitLab client: %w", err)
	}

	typeID, err := utils.ResolveTypeID(opts.workItemType)
	if err != nil {
		return err
	}

	createOpts := &gitlab.CreateWorkItemOptions{
		Title: opts.title,
	}

	if opts.description != "" {
		createOpts.Description = gitlab.Ptr(opts.description)
	}

	if opts.confidential {
		createOpts.Confidential = gitlab.Ptr(true)
	}

	fmt.Fprintln(opts.io.StdErr, "- Creating work item in", scope.Path)

	wi, _, err := client.WorkItems.CreateWorkItem(scope.Path, typeID, createOpts)
	if err != nil {
		return err
	}

	if opts.io.IsaTTY {
		fmt.Fprintf(opts.io.StdOut, "#%d %s\n%s\n", wi.IID, wi.Title, wi.WebURL)
	} else {
		fmt.Fprintln(opts.io.StdOut, wi.WebURL)
	}

	return nil

}
