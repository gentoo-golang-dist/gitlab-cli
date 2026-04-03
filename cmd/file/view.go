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

func NewCmdView(f *iostreams.IOStreams, cfg config.Config) *cobra.Command {
	var web bool
	var branch string
	var line int

	cmd := &cobra.Command{
		Use:   "view <file>",
		Short: "View a file from the repository",
		Long: `View a file from the repository.

Without --web flag, prints the file contents to the terminal.
With --web flag, opens the file in the browser.`,
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
				return viewFileWeb(gl, projID, filePath, branch, line)
			}

			return viewFileTerminal(gl, projID, filePath, branch)
		},
	}

	cmd.Flags().BoolVar(&web, "web", false, "Open file in browser")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name (defaults to default branch)")
	cmd.Flags().IntVar(&line, "line", 0, "Line number to jump to (with --web)")

	return cmd
}

func viewFileTerminal(gl *gitlab.Client, projID string, filePath string, branch string) error {
	opts := &gitlab.GetRawFileOptions{}
	if branch != "" {
		opts.Ref = gitlab.Ptr(branch)
	}

	content, _, err := gl.RepositoryFiles.GetRawFile(projID, filePath, opting...)
	if err != nil {
		return fmt.Errorf("failed to fetch file: %w", err)
	}

	fmt.Print(string(content))
	return nil
}

func viewFileWeb(gl *gitlab.Client, projID string, filePath string, branch string, line int) error {
	proj, _, err := gl.Projects.GetProject(projID, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	if branch == "" {
		branch = proj.DefaultBranch
	}

	url := fmt.Sprintf("%s/-/blob/%s/%s", proj.WebURL, branch, filePath)
	if line > 0 {
		url += fmt.Sprintf("#L%d", line)
	}

	return openInBrowser(url)
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd

	switch {
	case os.Getenv("BROWSER") != "":
		cmd = exec.Command(os.Getenv("BROWSER"), url)
	case os.Getenv("WSL_DISTRO_NAME") != "":
		cmd = exec.Command("wslview", url)
	default:
		switch {
		case os.Getenv("OSTYPE") == "darwin":
			cmd = exec.Command("open", url)
		case strings.Contains(os.Getenv("OSTYPE"), "linux"):
			cmd = exec.Command("xdg-open", url)
		case strings.Contains(os.Getenv("OS"), "Windows"):
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return fmt.Errorf("unsupported platform")
		}
	}

	return cmd.Run()
}
