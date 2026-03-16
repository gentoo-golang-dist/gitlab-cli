package note

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

// reviewEntry represents one review comment from the JSON input.
type reviewEntry struct {
	Body    string `json:"body"`
	File    string `json:"file,omitempty"`
	Line    string `json:"line,omitempty"`
	OldLine int    `json:"old_line,omitempty"`
	Reply   string `json:"reply,omitempty"`
	Resolve bool   `json:"resolve,omitempty"`
}

// NewCmdReview returns the `mr note review` command.
func NewCmdReview(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review [<mr-id> | <branch>]",
		Short: "Batch-create draft notes from JSON on stdin",
		Long: heredoc.Doc(`
			Read a JSON array of review comments from stdin and create them as
			draft notes. With --publish, immediately publish all drafts (submit review).

			Designed for programmatic use by editors, AI agents, and scripts.

			JSON array format — each object can have:

			- body (string, required): Note text in Markdown
			- file (string): File path for a diff comment
			- line (string or number): New-side line or range "N:M"; requires file
			- old_line (number): Old-side line; requires file
			- reply (string): Discussion ID or 8+ char prefix
			- resolve (bool): Resolve discussion on publish; requires reply

			Example JSON:
			  [
			    {"body": "General comment about the MR."},
			    {"file": "main.go", "line": "42", "body": "Unused variable."},
			    {"file": "main.go", "line": "10:15", "body": "Extract this."},
			    {"reply": "abc12345", "body": "Agreed.", "resolve": true},
			    {"file": "config.go", "old_line": 22, "body": "Why removed?"}
			  ]
		`),
		Example: heredoc.Doc(`
			# Create draft notes and publish (submit review)
			$ echo '[{"body":"LGTM"}]' | glab mr note review --publish

			# Create draft notes without publishing
			$ cat comments.json | glab mr note review 123

			# Submit a code review from a file
			$ glab mr note review --publish < review.json
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

			data, err := io.ReadAll(f.IO().In)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}

			var entries []reviewEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				return fmt.Errorf("invalid JSON input: %w", err)
			}
			if len(entries) == 0 {
				return fmt.Errorf("empty review: no comments provided")
			}

			project := repo.FullName()
			mrIID := mr.IID

			// Pre-fetch diff version if any entry has a file
			var version *gitlab.MergeRequestDiffVersion
			for _, e := range entries {
				if e.File != "" {
					version, err = mrutils.GetLatestDiffVersion(client, project, mrIID)
					if err != nil {
						return err
					}
					break
				}
			}

			var created []int64
			for i, e := range entries {
				if e.Body == "" {
					return fmt.Errorf("entry %d: empty body", i)
				}

				opts := &gitlab.CreateDraftNoteOptions{
					Note: gitlab.Ptr(e.Body),
				}

				if e.Reply != "" {
					discussionID, err := mrutils.ResolveDiscussionID(client, project, mrIID, e.Reply)
					if err != nil {
						return fmt.Errorf("entry %d: %w", i, err)
					}
					opts.InReplyToDiscussionID = gitlab.Ptr(discussionID)
					if e.Resolve {
						opts.ResolveDiscussion = gitlab.Ptr(true)
					}
				}

				if e.File != "" && version != nil {
					lineStart, lineEnd, err := mrutils.ParseLine(e.Line)
					if err != nil {
						return fmt.Errorf("entry %d: %w", i, err)
					}

					fileDiff, err := mrutils.FindFileDiff(version, e.File)
					if err != nil {
						return fmt.Errorf("entry %d: %w", i, err)
					}

					position, err := mrutils.BuildDiffPosition(version, fileDiff, lineStart, lineEnd, e.OldLine)
					if err != nil {
						return fmt.Errorf("entry %d: %w", i, err)
					}
					opts.Position = position
				}

				draft, _, err := client.DraftNotes.CreateDraftNote(project, mrIID, opts)
				if err != nil {
					return fmt.Errorf("entry %d: failed to create draft note: %w", i, err)
				}
				created = append(created, draft.ID)
				fmt.Fprintf(f.IO().StdOut, "Created draft note %d (%d/%d)\n", draft.ID, i+1, len(entries))
			}

			publish, _ := cmd.Flags().GetBool("publish")
			if publish {
				_, err := client.DraftNotes.PublishAllDraftNotes(project, mrIID)
				if err != nil {
					return fmt.Errorf("failed to publish draft notes: %w", err)
				}
				fmt.Fprintf(f.IO().StdOut, "Published %d draft notes\n", len(created))
			}

			return nil
		},
	}

	cmd.Flags().Bool("publish", false, "Publish all drafts after creating them (submit review).")

	return cmd
}
