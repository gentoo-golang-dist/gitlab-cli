package tag

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/tag/delete"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/tag/list"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/tag/view"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <command> [flags]",
		Short: "Manage container registry tags.",
		Long: heredoc.Doc(`
			List, view, and delete GitLab container registry tags.
		`),
		Aliases: []string{"tags"},
	}

	cmd.AddCommand(list.NewCmd(f))
	cmd.AddCommand(view.NewCmd(f))
	cmd.AddCommand(delete.NewCmd(f))

	return cmd
}
