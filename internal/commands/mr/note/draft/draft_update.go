package draft

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

// NewCmdDraftUpdate returns the `mr note draft update` command.
func NewCmdDraftUpdate(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [<mr-id> | <branch>] <draft-id> [-m <body>]",
		Short: "Update a draft note's body",
		Long: heredoc.Doc(`
			Update the body of a draft note by its numeric ID.

			The body can be provided via -m flag or stdin (when not a TTY).
		`),
		Example: heredoc.Doc(`
			# Update draft note 42 with a new body
			$ glab mr note draft update 42 -m "Revised comment"

			# Update draft note on MR 123
			$ glab mr note draft update 123 42 -m "New text"

			# Pipe new body from stdin
			$ echo "new text" | glab mr note draft update 42
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

			updated, _, err := client.DraftNotes.UpdateDraftNote(
				repo.FullName(),
				mr.IID,
				draftID,
				&gitlab.UpdateDraftNoteOptions{Note: gitlab.Ptr(body)},
			)
			if err != nil {
				return fmt.Errorf("failed to update draft note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "Updated draft note %d on !%d\n", updated.ID, mr.IID)
			return nil
		},
	}

	cmd.Flags().StringP("message", "m", "", "New body for the draft note.")

	return cmd
}
