package file

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	agentFileGetCmd "gitlab.com/gitlab-org/cli/internal/commands/agent/file/get"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdFile(f cmdutils.Factory) *cobra.Command {
	fileCmd := &cobra.Command{
		Use:   "file <command> [flags]",
		Short: "Read files from a repository. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
		Work with repository files at a specific ref without cloning the repository.

		Designed for agentic and CI workflows that need structured file access.
		`) + text.ExperimentalString,
	}

	fileCmd.AddCommand(agentFileGetCmd.NewCmdFileGet(f))

	return fileCmd
}
