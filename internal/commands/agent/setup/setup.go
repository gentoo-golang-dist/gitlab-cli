package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/skills"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io     *iostreams.IOStreams
	global bool
	path   string
}

func NewCmdSetup(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io: f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install glab's bundled agent skills. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Install glab's bundled SKILL.md files into your environment so AI
			coding agents can discover how to use glab.

			By default, skills are installed at project scope in '.agents/skills/'
			at the root of the current Git repository. This is the cross-agent
			standard directory and works with GitLab Duo, Claude Code, Codex,
			Gemini CLI, and any agent that follows the Agent Skills specification.

			Use '--global' to install at user scope in '~/.agents/skills/',
			making skills available across all projects and agents.

			Use '--path' to install to a custom directory.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Install skills in the current project (default)
			glab agent setup

			# Install skills globally (user scope)
			glab agent setup --global

			# Install skills to a custom directory
			glab agent setup --path /path/to/skills
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.global, "global", "g", false, "Install skills at user scope (~/.agents/skills/).")
	cmd.Flags().StringVar(&opts.path, "path", "", "Install skills to a custom directory.")

	return cmd
}

func runSetup(opts *options) error {
	targetDir, err := resolveTargetDir(opts)
	if err != nil {
		return err
	}

	installed, err := installSkills(targetDir)
	if err != nil {
		return err
	}

	c := opts.io.Color()
	for _, path := range installed {
		fmt.Fprintf(opts.io.StdOut, "%s Installed %s\n", c.GreenCheck(), path)
	}

	return nil
}

// resolveTargetDir determines where to install skills based on flags.
func resolveTargetDir(opts *options) (string, error) {
	if opts.path != "" {
		return opts.path, nil
	}

	if opts.global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		return filepath.Join(home, ".agents", "skills"), nil
	}

	// Default: project scope — skills/ at repo root
	repoRoot, err := git.ToplevelDir()
	if err != nil {
		return "", fmt.Errorf("not in a Git repository. Use --global or --path to specify a target: %w", err)
	}
	return filepath.Join(repoRoot, ".agents", "skills"), nil
}

// installSkills writes the bundled SKILL.md files to the target directory.
// Returns a list of file paths that were written.
func installSkills(targetDir string) ([]string, error) {
	var installed []string

	err := fs.WalkDir(skills.BundledSkills, "bundled", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "bundled/" prefix to get the relative install path
		relPath, err := filepath.Rel("bundled", path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		content, err := skills.BundledSkills.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading bundled skill %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", destPath, err)
		}

		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", destPath, err)
		}

		installed = append(installed, destPath)
		return nil
	})

	return installed, err
}
