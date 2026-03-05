package note

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

func NewCmdUpdate(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [<mr-id> | <branch>] <note-id> [-m <body>]",
		Short: "Update a note on a merge request",
		Long: heredoc.Doc(`
			Update the body of an existing note on a merge request.

			The note is located by its numeric ID across all discussions. The body can
			be provided via -m flag or stdin (when not a TTY).
		`),
		Example: heredoc.Doc(`
			# Update note 12345 with a new body
			$ glab mr note update 12345 -m "Updated comment"

			# Update note on MR 42
			$ glab mr note update 42 12345 -m "New text"

			# Pipe new body from stdin
			$ echo "New body" | glab mr note update 12345
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			// Last arg is always the note ID; preceding arg (if any) is MR ref.
			noteIDStr := args[len(args)-1]
			noteID, err := strconv.ParseInt(noteIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid note ID %q: %w", noteIDStr, err)
			}

			var mrArgs []string
			if len(args) == 2 {
				mrArgs = args[:1]
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, mrArgs, "any")
			if err != nil {
				return err
			}

			// Get body from -m flag or stdin
			body, _ := cmd.Flags().GetString("message")
			if strings.TrimSpace(body) == "" {
				if !f.IO().IsInTTY {
					data, err := io.ReadAll(f.IO().In)
					if err != nil {
						return fmt.Errorf("failed to read from stdin: %w", err)
					}
					body = strings.TrimSpace(string(data))
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("body is required: use -m or pipe via stdin")
			}

			// Find the note's discussion
			discussionID, _, err := mrutils.FindNoteInDiscussions(client, repo.FullName(), mr.IID, noteID)
			if err != nil {
				return err
			}

			updated, _, err := client.Discussions.UpdateMergeRequestDiscussionNote(
				repo.FullName(),
				mr.IID,
				discussionID,
				noteID,
				&gitlab.UpdateMergeRequestDiscussionNoteOptions{
					Body: &body,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to update note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, updated.ID)
			return nil
		},
	}

	cmd.Flags().StringP("message", "m", "", "New body for the note.")

	return cmd
}
