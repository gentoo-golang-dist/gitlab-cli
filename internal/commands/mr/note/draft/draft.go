package draft

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

// NewCmdDraft returns the `mr note draft` subcommand group.
func NewCmdDraft(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Manage draft (unpublished) notes on a merge request",
		Long: heredoc.Doc(`
			Create, list, update, delete, and publish draft notes on a merge request.

			Draft notes are visible only to you until published. They are used for
			the "review" workflow: accumulate comments, then publish them all at once.
		`),
	}

	cmd.AddCommand(NewCmdDraftCreate(f))
	cmd.AddCommand(NewCmdDraftList(f))
	cmd.AddCommand(NewCmdDraftUpdate(f))
	cmd.AddCommand(NewCmdDraftDelete(f))
	cmd.AddCommand(NewCmdDraftPublish(f))

	return cmd
}
