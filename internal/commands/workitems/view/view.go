package view

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	workitemsapi "gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/utils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)
	gitlabClient func() (*gitlab.Client, error)

	iid          string
	group        string
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		baseRepo:     f.BaseRepo,
		gitlabClient: f.GitLabClient,
	}

	cmd := &cobra.Command{
		Use:   "view <iid> [flags]",
		Short: "Show a single work item with its metadata. (EXPERIMENTAL)",
		Long: heredoc.Doc(`Show one work item in detail.

Scope comes from the current repo by default. Override with --group
for group-level items or -R for a different project.
`) + text.ExperimentalString,
		Example: heredoc.Doc(`
				# View a work item in the current project
				glab work-items view 42

				# View a group-level work item
				glab work-items view 7 -g gitlab-org

				# Full JSON output for scripting
				glab work-items view 42 --output json`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.iid = args[0]
			if err := opts.complete(cmd); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)
	cmd.Flags().StringP("group", "g", "", "View a work item in a group or subgroup")

	return cmd
}

func (opts *options) complete(cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	opts.group = group
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

	wi, err := workitemsapi.FetchWorkItem(ctx, client, scope, opts.iid)
	if err != nil {
		return err
	}

	switch opts.outputFormat {
	case "json":
		enc := json.NewEncoder(opts.io.StdOut)
		enc.SetIndent("", "  ")
		return enc.Encode(wi)
	case "text":
		return renderText(opts.io, wi)
	default:
		return fmt.Errorf("unsupported output format: %s", opts.outputFormat)
	}
}

// renderText prints identity first, metadata next, then description
// and children.
func renderText(streams *iostreams.IOStreams, wi *workitemsapi.WorkItem) error {
	c := streams.Color()
	w := streams.StdOut

	state := wi.State
	stateTint := c.Green
	if state == "CLOSED" {
		stateTint = c.Red
	}

	header := fmt.Sprintf("%s #%s · %s · %s", wi.WorkItemType.Name, wi.IID, stateTint(state), wi.WebURL)
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, c.Bold(wi.Title))
	fmt.Fprintln(w)

	kv := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(w, "%-12s %s\n", c.Gray(label+":"), value)
	}

	status := "-"
	if wi.Status != nil && wi.Status.Name != "" {
		status = wi.Status.Name
	}
	kv("Status", status)
	kv("Author", wi.Author.Username)

	if len(wi.Assignees.Nodes) > 0 {
		names := make([]string, len(wi.Assignees.Nodes))
		for i, a := range wi.Assignees.Nodes {
			names[i] = a.Username
		}
		kv("Assignees", strings.Join(names, ", "))
	}

	if len(wi.Labels.Nodes) > 0 {
		titles := make([]string, len(wi.Labels.Nodes))
		for i, l := range wi.Labels.Nodes {
			titles[i] = l.Title
		}
		kv("Labels", strings.Join(titles, ", "))
	}

	if wi.Milestone != nil {
		text := wi.Milestone.Title
		if wi.Milestone.DueDate != "" {
			text += " (due " + wi.Milestone.DueDate + ")"
		}
		kv("Milestone", text)
	}

	if wi.Parent != nil && wi.Parent.Title != "" {
		kv("Parent", fmt.Sprintf("#%s %s", wi.Parent.IID, wi.Parent.Title))
	}

	kv("Due date", wi.DueDate)
	kv("Start date", wi.StartDate)

	if wi.Namespace != nil {
		kv("Namespace", wi.Namespace.FullPath)
	}
	kv("Created", wi.CreatedAt)
	kv("Updated", wi.UpdatedAt)
	kv("Closed", wi.ClosedAt)

	if wi.Confidential {
		fmt.Fprintln(w, c.Yellow("Confidential"))
	}

	if wi.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, c.Bold("Description"))
		fmt.Fprintln(w, wi.Description)
	}

	if wi.Children != nil && wi.Children.Count > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s (%d)\n", c.Bold("Children"), wi.Children.Count)
		for _, child := range wi.Children.Nodes {
			tint := c.Green
			if child.State == "CLOSED" {
				tint = c.Red
			}
			fmt.Fprintf(w, "  %s #%s · %s · %s\n",
				child.WorkItemType.Name,
				tint(child.IID),
				child.Title,
				child.WebURL,
			)
		}
	}

	return nil
}
