package workitems

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/list"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdWorkItems(f cmdutils.Factory) *cobra.Command {
	workItemsCmd := &cobra.Command{
		Use:   "work-items <command> [flags]",
		Short: "Manage work items. (EXPERIMENTAL)",
		Long: heredoc.Doc(`Work with GitLab work items.

		Work items are the unified GitLab system for planning and tracking work, supporting
		various types including epics, issues, tasks, incidents, and test cases. Work items
		can be organized hierarchically to break down complex work into manageable pieces.
		`) + text.ExperimentalString,
	}

	// Register subcomands
	workItemsCmd.AddCommand(list.NewCmd(f))

	return workItemsCmd
}
