package draft

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

// NewCmdDraftList returns the `mr note draft list` command.
func NewCmdDraftList(f cmdutils.Factory) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list [<mr-id> | <branch>]",
		Short: "List your pending draft notes",
		Long: heredoc.Doc(`
			List all your unpublished draft notes on a merge request.

			Draft notes are private — you can only see your own.
		`),
		Example: heredoc.Doc(`
			# List draft notes on current branch's MR
			$ glab mr note draft list

			# List draft notes on MR 123
			$ glab mr note draft list 123

			# JSON output for scripting
			$ glab mr note draft list --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, args, "any")
			if err != nil {
				return err
			}

			drafts, _, err := client.DraftNotes.ListDraftNotes(repo.FullName(), mr.IID, nil)
			if err != nil {
				return fmt.Errorf("failed to list draft notes: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(f.IO().StdOut)
				enc.SetIndent("", "  ")
				return enc.Encode(drafts)
			}

			if len(drafts) == 0 {
				fmt.Fprintln(f.IO().StdOut, "No draft notes found.")
				return nil
			}

			for _, d := range drafts {
				printDraftNote(f, d)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON.")

	return cmd
}

func printDraftNote(f cmdutils.Factory, d *gitlab.DraftNote) {
	body := d.Note
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	body = strings.ReplaceAll(body, "\n", " ")

	loc := "general"
	if d.Position != nil && (d.Position.NewPath != "" || d.Position.OldPath != "") {
		path := d.Position.NewPath
		if path == "" {
			path = d.Position.OldPath
		}
		if d.Position.NewLine > 0 {
			loc = fmt.Sprintf("%s:%d", path, d.Position.NewLine)
		} else if d.Position.OldLine > 0 {
			loc = fmt.Sprintf("%s:~%d", path, d.Position.OldLine)
		} else {
			loc = path
		}
	} else if d.DiscussionID != "" {
		prefix := d.DiscussionID
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		loc = fmt.Sprintf("reply:%s", prefix)
	}
	fmt.Fprintf(f.IO().StdOut, "  %d  [%s]  %s\n", d.ID, loc, body)
}
