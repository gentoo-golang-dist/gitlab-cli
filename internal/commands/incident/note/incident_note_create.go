package note

import (
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	issuableCommentCmd "gitlab.com/gitlab-org/cli/internal/commands/issuable/comment"
)

// NewCmdNote creates a note command that delegates to comment add
// This maintains backward compatibility while eliminating code duplication.
// The note command is now an alias of comment, and direct usage routes to comment add.
func NewCmdNote(f cmdutils.Factory) *cobra.Command {
	commentCmd := issuableCommentCmd.NewCmdComment(f)
	// Find the add subcommand
	var addCmd *cobra.Command
	for _, subCmd := range commentCmd.Commands() {
		if subCmd.Name() == "add" {
			addCmd = subCmd
			break
		}
	}
	if addCmd == nil {
		// Fallback: create a new add command if not found
		addCmd = issuableCommentCmd.NewCmdAdd(f)
	}

	// When note is used directly (without subcommand), route to add
	noteCmd := &cobra.Command{
		Use:   "note <incident-id>",
		Short: "Comment on an incident in GitLab.",
		Long:  "This command is an alias for 'comment add'. Use 'glab incident comment' for more options.\n",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addCmd.RunE(cmd, args)
		},
	}
	// Copy flags from comment add
	noteCmd.Flags().StringP("message", "m", "", "Message text.")
	noteCmd.Flags().AddFlagSet(addCmd.Flags())
	return noteCmd
}
