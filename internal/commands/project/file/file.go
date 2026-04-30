package file

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	repoFileGetCmd "gitlab.com/gitlab-org/cli/internal/commands/project/file/get"
)

func NewCmdFile(f cmdutils.Factory) *cobra.Command {
	fileCmd := &cobra.Command{
		Use:   "file <command> [flags]",
		Short: "Read repository files over the API.",
		Long: heredoc.Doc(`
		Work with repository files at a specific ref without cloning the repository.

		Designed for agentic and CI workflows that need structured file access.
		`),
	}

	fileCmd.AddCommand(repoFileGetCmd.NewCmdFileGet(f))

	return fileCmd
}
