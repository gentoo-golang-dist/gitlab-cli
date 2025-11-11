package comment

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/issuable"
	issuableCommentCmd "gitlab.com/gitlab-org/cli/internal/commands/issuable/comment"
)

func NewCmdComment(f cmdutils.Factory) *cobra.Command {
	cmd := issuableCommentCmd.NewCmdComment(f)
	issueType := issuable.TypeIssue

	// Update command descriptions based on issueType
	cmd.Annotations = map[string]string{
		"issueType": string(issueType),
	}
	cmd.Short = fmt.Sprintf("Manage comments on %s", issueType)
	cmd.Long = fmt.Sprintf("List, add, and reply to comments on %s\n", issueType)
	cmd.Example = heredoc.Doc(fmt.Sprintf(`
		$ glab %[1]s comment list 123
		$ glab %[1]s comment add 123 -m "Great work!"
		$ glab %[1]s comment reply 123 456 -m "Thanks for the feedback"
	`, issueType)) + "\n"

	// Update child command descriptions
	for _, subCmd := range cmd.Commands() {
		switch subCmd.Name() {
		case "list":
			subCmd.Short = fmt.Sprintf("List comments on an %s", issueType)
			subCmd.Long = fmt.Sprintf("List all comments and notes on the specified %s\n", issueType)
		case "add":
			subCmd.Short = fmt.Sprintf("Add a comment to an %s", issueType)
			subCmd.Long = fmt.Sprintf("Add a new comment to the specified %s\n", issueType)
		case "reply":
			subCmd.Short = fmt.Sprintf("Reply to a comment on an %s", issueType)
			subCmd.Long = fmt.Sprintf("Reply to a specific comment on the specified %s\n", issueType)
		}
	}

	return cmd
}
