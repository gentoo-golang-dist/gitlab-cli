package note

import (
	"fmt"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/note/draft"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

func NewCmdNote(f cmdutils.Factory) *cobra.Command {
	mrCreateNoteCmd := &cobra.Command{
		Use:     "note [<id> | <branch>]",
		Aliases: []string{"comment"},
		Short:   "Add a comment or note to a merge request, or resolve/unresolve discussions.",
		Long:    ``,
		Example: heredoc.Doc(`
			# Add a comment to merge request with ID 123
			$ glab mr note 123 -m "Looks good to me!"

			# Add a comment to the merge request for the current branch
			$ glab mr note -m "LGTM"

			# Open your editor to compose a multi-line comment
			$ glab mr note 123

			# Pipe a comment from stdin
			$ echo "LGTM" | glab mr note 123

			# Add a diff comment on line 42 of main.go
			$ glab mr note 123 --file main.go --line 42 -m "Needs refactoring"

			# Add a diff comment on lines 10-15 (multiline range)
			$ glab mr note 123 --file main.go --line 10:15 -m "Extract this block"

			# Add a diff comment on a removed line (old side)
			$ glab mr note 123 --file main.go --old-line 7 -m "Why was this removed?"

			# Add a file-level diff comment (no line specified)
			$ glab mr note 123 --file main.go -m "General comment on this file"

			# Reply to an existing discussion (full or prefix ID)
			$ glab mr note 123 --reply abc12345 -m "I agree!"

			# Add a confidential internal note
			$ glab mr note 123 --internal -m "Internal feedback for maintainers"

			# Resolve a discussion by note ID
			$ glab mr note 123 --resolve 3107030349

			# Unresolve a discussion by note ID
			$ glab mr note 123 --unresolve 3107030349`),
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, args, "any")
			if err != nil {
				return err
			}

			// Check if we're resolving or unresolving
			resolveNoteID, _ := cmd.Flags().GetInt64("resolve")
			unresolveNoteID, _ := cmd.Flags().GetInt64("unresolve")

			if resolveNoteID != 0 {
				return resolveDiscussion(client, f, mr, repo, resolveNoteID, true)
			}

			if unresolveNoteID != 0 {
				return resolveDiscussion(client, f, mr, repo, unresolveNoteID, false)
			}

			// Check if we're replying to an existing discussion
			replyTo, _ := cmd.Flags().GetString("reply")

			// Create note (existing behavior)
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

					err = f.IO().Editor(cmd.Context(), &body, "Note message:", "Enter the note message for the merge request.", "", editor)
					if err != nil {
						return err
					}
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("aborted... Note has an empty message.")
			}

			uniqueNoteEnabled, _ := cmd.Flags().GetBool("unique")

			if uniqueNoteEnabled {
				opts := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
				for {
					notes, resp, err := client.Notes.ListMergeRequestNotes(repo.FullName(), mr.IID, opts)
					if err != nil {
						return fmt.Errorf("running merge request note deduplication: %v", err)
					}
					for _, noteInfo := range notes {
						if noteInfo.Body == strings.TrimSpace(body) {
							fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, noteInfo.ID)
							return nil
						}
					}
					if resp == nil || resp.NextPage == 0 {
						break
					}
					opts.Page = resp.NextPage
				}
			}

			// Reply to an existing discussion
			if replyTo != "" {
				discussionID, err := mrutils.ResolveDiscussionID(client, repo.FullName(), mr.IID, replyTo)
				if err != nil {
					return err
				}

				note, _, err := client.Discussions.AddMergeRequestDiscussionNote(
					repo.FullName(),
					mr.IID,
					discussionID,
					&gitlab.AddMergeRequestDiscussionNoteOptions{Body: &body},
				)
				if err != nil {
					return fmt.Errorf("failed to add reply: %w", err)
				}

				fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, note.ID)
				return nil
			}

			internal, _ := cmd.Flags().GetBool("internal")

			// Internal notes use the flat Notes API (Discussions API doesn't support internal)
			if internal {
				note, _, err := client.Notes.CreateMergeRequestNote(repo.FullName(), mr.IID, &gitlab.CreateMergeRequestNoteOptions{
					Body:     &body,
					Internal: gitlab.Ptr(true),
				})
				if err != nil {
					return err
				}
				fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, note.ID)
				return nil
			}

			filePath, _ := cmd.Flags().GetString("file")

			createOpts := &gitlab.CreateMergeRequestDiscussionOptions{Body: &body}

			if filePath != "" {
				lineFlag, _ := cmd.Flags().GetString("line")
				oldLine, _ := cmd.Flags().GetInt("old-line")

				lineStart, lineEnd, err := mrutils.ParseLine(lineFlag)
				if err != nil {
					return err
				}

				version, err := mrutils.GetLatestDiffVersion(client, repo.FullName(), mr.IID)
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

				createOpts.Position = position
			}

			disc, _, err := client.Discussions.CreateMergeRequestDiscussion(repo.FullName(), mr.IID, createOpts)
			if err != nil {
				return err
			}

			fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, disc.Notes[0].ID)
			return nil
		},
	}

	mrCreateNoteCmd.Flags().StringP("message", "m", "", "Comment or note message.")
	mrCreateNoteCmd.Flags().Bool("unique", false, "Don't create a comment or note if it already exists.")
	mrCreateNoteCmd.Flags().Int64("resolve", 0, "Resolve the discussion containing the specified note ID.")
	mrCreateNoteCmd.Flags().Int64("unresolve", 0, "Unresolve the discussion containing the specified note ID.")
	mrCreateNoteCmd.Flags().String("reply", "", "Reply to an existing discussion by ID (full or 8+ char prefix).")
	mrCreateNoteCmd.Flags().Bool("internal", false, "Create a confidential internal note (uses flat Notes API).")
	mrCreateNoteCmd.Flags().String("file", "", "File path for a diff comment (targets the latest MR diff version).")
	mrCreateNoteCmd.Flags().String("line", "", "Line in the new version: a single number or a range N:M.")
	mrCreateNoteCmd.Flags().Int("old-line", 0, "Line in the old version (for commenting on removed lines).")

	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("message", "resolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("message", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("resolve", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("file", "resolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("file", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("reply", "file")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("reply", "resolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("reply", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("internal", "file")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("internal", "reply")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("internal", "resolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("internal", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("line", "old-line")

	mrCreateNoteCmd.AddCommand(NewCmdList(f))
	mrCreateNoteCmd.AddCommand(NewCmdResolve(f))
	mrCreateNoteCmd.AddCommand(NewCmdUnresolve(f))
	mrCreateNoteCmd.AddCommand(NewCmdUpdate(f))
	mrCreateNoteCmd.AddCommand(NewCmdDelete(f))
	mrCreateNoteCmd.AddCommand(NewCmdReview(f))
	mrCreateNoteCmd.AddCommand(draft.NewCmdDraft(f))

	return mrCreateNoteCmd
}

func resolveDiscussion(client *gitlab.Client, f cmdutils.Factory, mr *gitlab.MergeRequest, repo glrepo.Interface, noteID int64, resolve bool) error {
	targetDiscussionID, _, err := mrutils.FindNoteInDiscussions(client, repo.FullName(), mr.IID, noteID)
	if err != nil {
		return err
	}

	action := "resolve"
	if !resolve {
		action = "unresolve"
	}

	_, _, err = client.Discussions.ResolveMergeRequestDiscussion(
		repo.FullName(),
		mr.IID,
		targetDiscussionID,
		&gitlab.ResolveMergeRequestDiscussionOptions{
			Resolved: &resolve,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to %s discussion: %w", action, err)
	}

	fmt.Fprintf(f.IO().StdOut, "✓ Discussion %sd (note #%d in !%d)\n", action, noteID, mr.IID)
	return nil
}
