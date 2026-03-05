package draft

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

// NewCmdDraftPublish returns the `mr note draft publish` command.
func NewCmdDraftPublish(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish [<mr-id> | <branch>] [<draft-id>]",
		Short: "Publish draft note(s)",
		Long: heredoc.Doc(`
			Publish draft notes, making them visible to everyone.

			Publish a single draft by ID, or all pending drafts with --all.
			Publishing all drafts is the equivalent of "Submit review" in the
			GitLab web UI — all drafts become real notes in a single operation.
		`),
		Example: heredoc.Doc(`
			# Publish a single draft note
			$ glab mr note draft publish 123

			# Publish draft note on specific MR
			$ glab mr note draft publish 42 123

			# Publish all draft notes (submit review)
			$ glab mr note draft publish --all

			# Publish all draft notes on specific MR
			$ glab mr note draft publish 42 --all
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			publishAll, _ := cmd.Flags().GetBool("all")

			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			// Parse args: with --all we expect 0 or 1 args (MR ref).
			// Without --all we expect 1 or 2 args (optional MR ref + draft ID).
			var mrArgs []string
			var draftID int64

			if publishAll {
				// All positional args are MR ref
				mrArgs = args
			} else {
				if len(args) == 0 {
					return fmt.Errorf("specify a draft note ID or use --all")
				}
				// Last arg is the draft ID
				draftIDStr := args[len(args)-1]
				var parseErr error
				draftID, parseErr = strconv.ParseInt(draftIDStr, 10, 64)
				if parseErr != nil {
					return fmt.Errorf("invalid draft note ID %q: %w", draftIDStr, parseErr)
				}
				if len(args) == 2 {
					mrArgs = args[:1]
				}
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, mrArgs, "any")
			if err != nil {
				return err
			}

			if publishAll {
				_, err := client.DraftNotes.PublishAllDraftNotes(repo.FullName(), int64(mr.IID))
				if err != nil {
					return fmt.Errorf("failed to publish all draft notes: %w", err)
				}
				fmt.Fprintf(f.IO().StdOut, "✓ Published all draft notes on !%d\n", mr.IID)
				return nil
			}

			_, err = client.DraftNotes.PublishDraftNote(repo.FullName(), int64(mr.IID), draftID)
			if err != nil {
				return fmt.Errorf("failed to publish draft note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "✓ Published draft note %d on !%d\n", draftID, mr.IID)
			return nil
		},
	}

	cmd.Flags().Bool("all", false, "Publish all draft notes.")

	return cmd
}
