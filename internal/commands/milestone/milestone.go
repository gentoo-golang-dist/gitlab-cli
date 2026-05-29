package milestone

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	cmdCreate "gitlab.com/gitlab-org/cli/internal/commands/milestone/create"
	cmdDelete "gitlab.com/gitlab-org/cli/internal/commands/milestone/delete"
	cmdEdit "gitlab.com/gitlab-org/cli/internal/commands/milestone/edit"
	cmdGet "gitlab.com/gitlab-org/cli/internal/commands/milestone/get"
	cmdList "gitlab.com/gitlab-org/cli/internal/commands/milestone/list"
)

func NewCmdMilestone(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone <command>",
		Short: "Manage group or project milestones.",
		Long: heredoc.Doc(`
		Milestones group issues, merge requests, and epics around a shared
		goal or release. Use the subcommands to create, list, edit, delete,
		or look up milestones in the current project, in another project, or
		in a group.
		`),
	}

	cmdutils.EnableRepoOverride(cmd, f)

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdEdit.NewCmdEdit(f))
	cmd.AddCommand(cmdGet.NewCmdGet(f))
	cmd.AddCommand(cmdList.NewCmdList(f))

	return cmd
}
