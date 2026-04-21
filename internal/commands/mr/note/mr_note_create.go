package note

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [<id> | <branch>]",
		Short: "Create a comment or discussion on a merge request. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Add a comment to a merge request. The command creates the comment as a new
			discussion thread.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Add a comment to merge request 123
			glab mr note create 123 -m "Looks good to me!"

			# Add a comment to the current branch's merge request
			glab mr note create -m "LGTM"

			# Open editor to compose the message
			glab mr note create 123

			# Pipe from stdin
			echo "LGTM" | glab mr note create 123

			# Skip if already posted
			glab mr note create 123 -m "LGTM" --unique
		`),
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

			body, _ := cmd.Flags().GetString("message")
			if strings.TrimSpace(body) == "" {
				body, err = getBodyFromStdinOrEditor(f, cmd)
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("aborted... Note has an empty message.")
			}

			uniqueNoteEnabled, _ := cmd.Flags().GetBool("unique")
			if uniqueNoteEnabled {
				found, err := deduplicateNote(client, repo.FullName(), mr.IID, body, mr.WebURL, f.IO().StdOut)
				if err != nil {
					return err
				}
				if found {
					return nil
				}
			}

			disc, _, err := client.Discussions.CreateMergeRequestDiscussion(
				repo.FullName(),
				mr.IID,
				&gitlab.CreateMergeRequestDiscussionOptions{Body: &body},
				gitlab.WithContext(cmd.Context()),
			)
			if err != nil {
				return err
			}

			if len(disc.Notes) == 0 {
				return fmt.Errorf("discussion created but returned no notes")
			}

			fmt.Fprintf(f.IO().StdOut, "%s#note_%d\n", mr.WebURL, disc.Notes[0].ID)
			return nil
		},
	}

	cmd.Flags().StringP("message", "m", "", "Comment or note message.")
	cmd.Flags().Bool("unique", false, "Don't create a note if note with same body already exists. Reads all MR comments first.")

	return cmd
}
