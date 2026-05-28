package repository

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/repository/delete"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/repository/list"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/repository/view"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository <command> [flags]",
		Short: "Manage container registry repositories.",
		Long: heredoc.Doc(`
			List, view, and delete GitLab container registry repositories.
		`),
	}

	cmd.AddCommand(list.NewCmd(f))
	cmd.AddCommand(view.NewCmd(f))
	cmd.AddCommand(delete.NewCmd(f))

	return cmd
}
