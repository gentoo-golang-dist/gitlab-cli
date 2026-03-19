package note

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

type listOptions struct {
	factory cmdutils.Factory

	args         []string
	noteType     string
	state        string
	filePath     string
	outputFormat string
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &listOptions{
		factory: f,
	}

	cmd := &cobra.Command{
		Use:   "list [<id> | <branch>]",
		Short: "List discussions on a merge request (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			This command is experimental.

			Fetch and display all discussions on a merge request.

			Uses the same output format as 'glab mr view --comments'.
			Supports filtering by note type, resolution state, and file path.
			Supports JSON output for scripting.
		`),
		Example: heredoc.Doc(`
			# List all discussions on the current branch's MR
			glab mr note list

			# List diff comments only
			glab mr note list --type diff

			# List unresolved discussions
			glab mr note list --state unresolved

			# List discussions on a specific file
			glab mr note list --file src/main.go

			# JSON output for scripting
			glab mr note list -F json | jq '.[].notes[].body'

			# List discussions on MR 123
			glab mr note list 123`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().VarP(
		cmdutils.NewEnumValue([]string{"all", "general", "diff", "system"}, "all", &opts.noteType),
		"type", "t", "Note type: all, general, diff, system.",
	)
	cmd.Flags().Var(
		cmdutils.NewEnumValue([]string{"all", "resolved", "unresolved"}, "all", &opts.state),
		"state", "Resolution state: all, resolved, unresolved.",
	)
	cmd.Flags().StringVar(&opts.filePath, "file", "", "Show only diff notes on this file path.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *listOptions) complete(args []string) {
	o.args = args
}

func (o *listOptions) run(ctx context.Context) error {
	client, err := o.factory.GitLabClient()
	if err != nil {
		return err
	}

	mr, repo, err := mrutils.MRFromArgs(ctx, o.factory, o.args, "any")
	if err != nil {
		return err
	}

	discussions, err := mrutils.ListAllDiscussions(client, repo.FullName(), mr.IID, &gitlab.ListMergeRequestDiscussionsOptions{})
	if err != nil {
		return err
	}

	filterOpts := mrutils.FilterOpts{}
	if o.noteType != "all" {
		filterOpts.Type = o.noteType
	}
	if o.state != "all" {
		filterOpts.State = o.state
	}
	filterOpts.FilePath = o.filePath

	filtered := mrutils.FilterDiscussions(discussions, filterOpts)

	if o.outputFormat == "json" {
		enc := json.NewEncoder(o.factory.IO().StdOut)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}

	out := o.factory.IO().StdOut
	if len(filtered) == 0 {
		fmt.Fprintln(out, "No discussions found.")
		return nil
	}

	showSystemLogs := o.noteType == "system"

	if o.factory.IO().IsOutputTTY() {
		mrutils.PrintDiscussionsTTY(out, o.factory.IO(), filtered, showSystemLogs)
	} else {
		mrutils.PrintDiscussionsRaw(out, filtered, showSystemLogs)
	}

	return nil
}
