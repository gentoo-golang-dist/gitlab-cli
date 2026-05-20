package container_registry

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/repository"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/tag"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container-registry <command> [flags]",
		Short: "Work with GitLab container registries.",
		Long: heredoc.Doc(`
			List and manage GitLab container registry repositories and tags.
		`),
		Aliases: []string{"registry"},
		Example: heredoc.Doc(`
			# List container registry repositories for the current project
			glab container-registry repository list

			# List tags for a container registry repository
			glab container-registry tag list 123

			# Delete a container registry tag
			glab container-registry tag delete 123 latest`),
	}

	cmdutils.EnableRepoOverride(cmd, f)

	cmd.AddCommand(repository.NewCmd(f))
	cmd.AddCommand(tag.NewCmd(f))

	return cmd
}
