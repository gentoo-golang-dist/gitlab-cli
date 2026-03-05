package draft

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

// NewCmdDraftDelete returns the `mr note draft delete` command.
func NewCmdDraftDelete(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [<mr-id> | <branch>] <draft-id> [--yes]",
		Short: "Delete a draft note",
		Long: heredoc.Doc(`
			Delete a draft note by its numeric ID.

			A confirmation prompt is shown unless --yes is provided.
		`),
		Example: heredoc.Doc(`
			# Delete draft note 42 (with confirmation prompt)
			$ glab mr note draft delete 42

			# Delete draft note on MR 123 without confirmation
			$ glab mr note draft delete 123 42 --yes

			# Skip confirmation with short flag
			$ glab mr note draft delete 42 -y
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			// Last arg is always the draft ID; preceding arg (if any) is MR ref.
			draftIDStr := args[len(args)-1]
			draftID, err := strconv.ParseInt(draftIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid draft note ID %q: %w", draftIDStr, err)
			}

			var mrArgs []string
			if len(args) == 2 {
				mrArgs = args[:1]
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, mrArgs, "any")
			if err != nil {
				return err
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				if !f.IO().IsInteractive() {
					return fmt.Errorf("cannot prompt for confirmation (not a TTY): use --yes to skip")
				}
				var confirmed bool
				if err := f.IO().Confirm(cmd.Context(), &confirmed, fmt.Sprintf("Delete draft note %d from !%d?", draftID, mr.IID)); err != nil {
					return err
				}
				if !confirmed {
					return fmt.Errorf("aborted")
				}
			}

			_, err = client.DraftNotes.DeleteDraftNote(repo.FullName(), mr.IID, draftID)
			if err != nil {
				return fmt.Errorf("failed to delete draft note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "✓ Draft note %d deleted from !%d\n", draftID, mr.IID)
			return nil
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt.")

	return cmd
}
