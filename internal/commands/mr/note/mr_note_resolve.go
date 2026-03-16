package note

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

func NewCmdResolve(f cmdutils.Factory) *cobra.Command {
	return newResolveCmd(f, true)
}

func NewCmdUnresolve(f cmdutils.Factory) *cobra.Command {
	return newResolveCmd(f, false)
}

func newResolveCmd(f cmdutils.Factory, resolve bool) *cobra.Command {
	action := "resolve"
	if !resolve {
		action = "unresolve"
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [<mr-id> | <branch>] <discussion-id>", action),
		Short: fmt.Sprintf("%s a discussion on a merge request", capitalize(action)),
		Long: heredoc.Docf(`
			%s a discussion on a merge request by discussion ID.

			The discussion ID can be the full 40-character hex string or an 8+ character
			prefix. If the prefix matches multiple discussions, an error is returned with
			the ambiguous matches (exit code 3).
		`, capitalize(action)),
		Example: heredoc.Docf(`
			# %s a discussion by full ID
			$ glab mr note %s abc12345deadbeef1234567890abcdef12345678

			# %s a discussion by prefix (8+ chars)
			$ glab mr note %s abc12345

			# %s a discussion on MR 123
			$ glab mr note %s 123 abc12345
		`, capitalize(action), action, capitalize(action), action, capitalize(action), action),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			// Last arg is always the discussion ID; preceding arg (if any) is MR ref.
			var mrArgs []string
			discussionPrefix := args[len(args)-1]
			if len(args) == 2 {
				mrArgs = args[:1]
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, mrArgs, "any")
			if err != nil {
				return err
			}

			discussionID, err := mrutils.ResolveDiscussionID(client, repo.FullName(), mr.IID, discussionPrefix)
			if err != nil {
				return err
			}

			_, _, err = client.Discussions.ResolveMergeRequestDiscussion(
				repo.FullName(),
				mr.IID,
				discussionID,
				&gitlab.ResolveMergeRequestDiscussionOptions{
					Resolved: &resolve,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to %s discussion: %w", action, err)
			}

			fmt.Fprintf(f.IO().StdOut, "✓ Discussion %sd (%s in !%d)\n", action, discussionID[:8], mr.IID)
			return nil
		},
	}

	return cmd
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
