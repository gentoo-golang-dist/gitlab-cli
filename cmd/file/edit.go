package file

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

func NewCmdEdit(f *iostreams.IOStreams, cfg config.Config) *cobra.Command {
	var web bool
	var branch string

	cmd := &cobra.Command{
		Use:   "edit <file>",
		Short: "Edit a file from the repository",
		Long: `Edit a file from the repository.

Without --web flag, opens the file in your $EDITOR.
With --web flag, opens the file in GitLab Web IDE.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			gl, err := glinstance.GetClient()
			if err != nil {
				return err
			}

			projID, err := glinstance.GetCurrentProjectID()
			if err != nil {
				return err
			}

			if web {
				return editFileWeb(gl, projID, filePath, branch)
			}

			return editFileLocal(gl, projID, filePath, branch)
		},
	}

	cmd.Flags().BoolVar(&web, "web", false, "Open file in GitLab Web IDE")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name (defaults to default branch)")

	return cmd
}

func editFileLocal(gl *gitlab.Client, projID string, filePath string, branch string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// For local editing, we'd need to clone/fetch the file
	// This is a simplified version that just opens the editor
	// In a real implementation, you might want to fetch the file first
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func editFileWeb(gl *gitlab.Client, projID string, filePath string, branch string) error {
	proj, _, err := gl.Projects.GetProject(projID, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	if branch == "" {
		branch = proj.DefaultBranch
	}

	// Open Web IDE for the file
	url := fmt.Sprintf("%s/-/edit/%s/%s", proj.WebURL, branch, filePath)

	return openInBrowser(url)
}
