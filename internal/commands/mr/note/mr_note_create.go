package note

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
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

			// Create note (existing behavior)
			body, _ := cmd.Flags().GetString("message")

			if strings.TrimSpace(body) == "" {
				editor, err := cmdutils.GetEditor(f.Config)
				if err != nil {
					return err
				}

				err = f.IO().Editor(cmd.Context(), &body, "Note message:", "Enter the note message for the merge request.", "", editor)
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("aborted... Note has an empty message.")
			}

			uniqueNoteEnabled, _ := cmd.Flags().GetBool("unique")

			if uniqueNoteEnabled {
				opts := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: api.DefaultListLimit}}
				notes, _, err := client.Notes.ListMergeRequestNotes(repo.FullName(), mr.IID, opts)
				if err != nil {
					return fmt.Errorf("running merge request note deduplication: %v", err)
				}
				for _, noteInfo := range notes {
					if noteInfo.Body == strings.TrimSpace(body) {
						fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, noteInfo.ID)
						return nil
					}
				}
			}

			noteInfo, _, err := client.Notes.CreateMergeRequestNote(repo.FullName(), mr.IID, &gitlab.CreateMergeRequestNoteOptions{Body: &body})
			if err != nil {
				return err
			}

			fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, noteInfo.ID)
			return nil
		},
	}

	mrCreateNoteCmd.Flags().StringP("message", "m", "", "Comment or note message.")
	mrCreateNoteCmd.Flags().Bool("unique", false, "Don't create a comment or note if it already exists.")
	mrCreateNoteCmd.Flags().Int64("resolve", 0, "Resolve the discussion containing the specified note ID.")
	mrCreateNoteCmd.Flags().Int64("unresolve", 0, "Unresolve the discussion containing the specified note ID.")

	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("message", "resolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("message", "unresolve")
	mrCreateNoteCmd.MarkFlagsMutuallyExclusive("resolve", "unresolve")

	return mrCreateNoteCmd
}

func resolveDiscussion(client *gitlab.Client, f cmdutils.Factory, mr *gitlab.MergeRequest, repo glrepo.Interface, noteID int64, resolve bool) error {
	// List all discussions to find the one containing this note (with pagination)
	var allDiscussions []*gitlab.Discussion
	var page int64 = 1
	for {
		discussions, resp, err := client.Discussions.ListMergeRequestDiscussions(
			repo.FullName(),
			mr.IID,
			&gitlab.ListMergeRequestDiscussionsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: 100,
				},
			},
		)
		if err != nil {
			return fmt.Errorf("failed to list discussions: %w", err)
		}
		allDiscussions = append(allDiscussions, discussions...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	// Find discussion containing the note
	var targetDiscussionID string
	var found bool

	for _, discussion := range allDiscussions {
		for _, note := range discussion.Notes {
			if note.ID == noteID {
				targetDiscussionID = discussion.ID
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("note %d not found in merge request !%d", noteID, mr.IID)
	}

	// Resolve or unresolve the discussion
	action := "resolve"
	if !resolve {
		action = "unresolve"
	}

	_, _, err := client.Discussions.ResolveMergeRequestDiscussion(
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
