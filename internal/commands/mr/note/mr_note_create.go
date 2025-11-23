package note

import (
	"fmt"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	mrCommentCmd "gitlab.com/gitlab-org/cli/internal/commands/mr/comment"

	"github.com/spf13/cobra"
)

// NewCmdNote creates a note command that delegates to comment add
// This maintains backward compatibility while eliminating code duplication.
// The note command is now an alias of comment, and direct usage routes to comment add.
func NewCmdNote(f cmdutils.Factory) *cobra.Command {
	commentCmd := mrCommentCmd.NewCmdComment(f)
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
		addCmd = mrCommentCmd.NewCmdAdd(f)
	}

	// When note is used directly (without subcommand), route to add
	noteCmd := &cobra.Command{
		Use:   "note [<id> | <branch>]",
		Short: "Add a comment or note to a merge request.",
		Long:  "This command is an alias for 'comment add'. Use 'glab mr comment' for more options.\n",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Copy flag values from noteCmd to addCmd so the bound variables are updated
			if cmd.Flags().Changed("message") {
				msg, _ := cmd.Flags().GetString("message")
				_ = addCmd.Flags().Set("message", msg)
			}
			if cmd.Flags().Changed("unique") {
				unique, _ := cmd.Flags().GetBool("unique")
				_ = addCmd.Flags().Set("unique", fmt.Sprintf("%t", unique))
			}
			return addCmd.RunE(addCmd, args)
		},
	}
	// Copy flags from comment add
	noteCmd.Flags().StringP("message", "m", "", "Comment or note message.")
	noteCmd.Flags().Bool("unique", false, "Don't create a comment or note if it already exists.")
	noteCmd.Flags().AddFlagSet(addCmd.Flags())
	return noteCmd
}
