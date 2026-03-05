package draft

import (
	"fmt"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

// NewCmdDraftCreate returns the `mr note draft create` command.
func NewCmdDraftCreate(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [<mr-id> | <branch>]",
		Short: "Create a draft note on a merge request",
		Long: heredoc.Doc(`
			Create a draft note on a merge request. Supports general notes,
			diff comments (--file, --line, --old-line), and replies (--reply).

			The draft remains unpublished until you run 'glab mr note draft publish'.

			The --resolve flag (only with --reply) auto-resolves the target
			discussion when the draft is published.

			Note: GitLab allows only one draft reply per discussion per user.
		`),
		Example: heredoc.Doc(`
			# Create a general draft note
			$ glab mr note draft create 123 -m "Needs work"

			# Create a diff draft note on line 42
			$ glab mr note draft create 123 --file main.go --line 42 -m "Nit: rename"

			# Create a draft reply to an existing discussion
			$ glab mr note draft create 123 --reply abc12345 -m "Good point"

			# Create a draft reply that resolves the discussion on publish
			$ glab mr note draft create 123 --resolve --reply abc12345 -m "Fixed"

			# Pipe body from stdin
			$ echo "LGTM" | glab mr note draft create 123
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

			body, _ := cmd.Flags().GetString("message")
			if strings.TrimSpace(body) == "" {
				if !f.IO().IsInTTY {
					data, err := io.ReadAll(f.IO().In)
					if err != nil {
						return fmt.Errorf("failed to read from stdin: %w", err)
					}
					body = strings.TrimSpace(string(data))
				} else {
					editor, err := cmdutils.GetEditor(f.Config)
					if err != nil {
						return err
					}
					err = f.IO().Editor(cmd.Context(), &body, "Draft note message:", "Enter the draft note message for the merge request.", "", editor)
					if err != nil {
						return err
					}
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("aborted... Draft note has an empty message.")
			}

			opts := &gitlab.CreateDraftNoteOptions{
				Note: gitlab.Ptr(body),
			}

			// Reply to existing discussion
			replyTo, _ := cmd.Flags().GetString("reply")
			if replyTo != "" {
				discussionID, err := mrutils.ResolveDiscussionID(client, repo.FullName(), int64(mr.IID), replyTo)
				if err != nil {
					return err
				}
				opts.InReplyToDiscussionID = gitlab.Ptr(discussionID)

				resolve, _ := cmd.Flags().GetBool("resolve")
				if resolve {
					opts.ResolveDiscussion = gitlab.Ptr(true)
				}
			}

			// Diff note position
			filePath, _ := cmd.Flags().GetString("file")
			if filePath != "" {
				lineFlag, _ := cmd.Flags().GetString("line")
				oldLine, _ := cmd.Flags().GetInt("old-line")

				lineStart, lineEnd, err := mrutils.ParseLine(lineFlag)
				if err != nil {
					return err
				}

				version, err := mrutils.GetLatestDiffVersion(client, repo.FullName(), int64(mr.IID))
				if err != nil {
					return err
				}

				fileDiff, err := mrutils.FindFileDiff(version, filePath)
				if err != nil {
					return err
				}

				position, err := mrutils.BuildDiffPosition(version, fileDiff, lineStart, lineEnd, oldLine)
				if err != nil {
					return err
				}

				opts.Position = position
			}

			draft, _, err := client.DraftNotes.CreateDraftNote(repo.FullName(), int64(mr.IID), opts)
			if err != nil {
				return fmt.Errorf("failed to create draft note: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "Created draft note %d on !%d\n", draft.ID, mr.IID)
			return nil
		},
	}

	cmd.Flags().StringP("message", "m", "", "Note body text.")
	cmd.Flags().String("file", "", "File path for a diff comment.")
	cmd.Flags().String("line", "", "Line in new version: single (42) or range (10:15).")
	cmd.Flags().Int("old-line", 0, "Line in old version (for removed lines).")
	cmd.Flags().String("reply", "", "Discussion ID or 8+ char prefix to reply to.")
	cmd.Flags().Bool("resolve", false, "Resolve the discussion when this draft is published (only with --reply).")

	cmd.MarkFlagsMutuallyExclusive("file", "reply")
	cmd.MarkFlagsMutuallyExclusive("line", "old-line")
	cmd.MarkFlagsMutuallyExclusive("resolve", "file")

	return cmd
}
