package note

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

func NewCmdDelete(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [<mr-id> | <branch>] <note-id> [--yes]",
		Short: "Delete a note from a merge request",
		Long: heredoc.Doc(`
			Delete an existing note from a merge request by its numeric note ID.

			The note is located across all discussions. A confirmation prompt is shown
			unless --yes is provided.
		`),
		Example: heredoc.Doc(`
			# Delete note 12345 (with confirmation prompt)
			$ glab mr note delete 12345

			# Delete note on MR 42 without confirmation
			$ glab mr note delete 42 12345 --yes

			# Delete note on MR identified by branch
			$ glab mr note delete feature-branch 12345 -y
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

			// Find the note's discussion
			discussionID, _, err := mrutils.FindNoteInDiscussions(client, repo.FullName(), mr.IID, noteID)
			if err != nil {
				return err
			}

			// Confirmation prompt unless --yes
			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				if !f.IO().IsInteractive() {
					return fmt.Errorf("cannot prompt for confirmation (not a TTY): use --yes to skip")
				}
				var confirmed bool
				if err := f.IO().Confirm(cmd.Context(), &confirmed, fmt.Sprintf("Delete note %d from !%d?", noteID, mr.IID)); err != nil {
					return err
				}
				if !confirmed {
					return fmt.Errorf("aborted")
				}
			}

			_, err = client.Discussions.DeleteMergeRequestDiscussionNote(
				repo.FullName(),
				int64(mr.IID),
				discussionID,
				noteID,
			)
			if err != nil {
				return fmt.Errorf("failed to delete note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "✓ Note %d deleted from !%d\n", noteID, mr.IID)
			return nil
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt.")

	return cmd
}
