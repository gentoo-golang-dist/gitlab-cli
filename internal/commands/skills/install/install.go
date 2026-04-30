package install

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
	io        *iostreams.IOStreams
	global    bool
	path      string
	force     bool
	targetDir string
}

func NewCmdInstall(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io: f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "install",
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

			Use '--path' to install to a custom directory. The path is resolved
			relative to the current working directory, not the repository root.

			Existing skill files are not overwritten unless '--force' is specified.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Install skills in the current project (default)
			glab skills install

			# Install skills globally (user scope)
			glab skills install --global

			# Install skills to a custom directory
			glab skills install --path /path/to/skills

			# Overwrite existing skill files
			glab skills install --force
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	fl := cmd.Flags()
	fl.BoolVarP(&opts.global, "global", "g", false, "Install skills at user scope (~/.agents/skills/). (default false)")
	fl.StringVar(&opts.path, "path", "", "Install skills to a custom <directory>.")
	fl.BoolVarP(&opts.force, "force", "f", false, "Overwrite existing skill files. (default false)")
	cmd.MarkFlagsMutuallyExclusive("global", "path")

	return cmd
}

// complete resolves the target directory based on flags and stores it on opts.
func (opts *options) complete() error {
	dir, err := resolveTargetDir(opts)
	if err != nil {
		return err
	}
	opts.targetDir = dir
	return nil
}

// run executes the command's business logic.
func (opts *options) run() error {
	// Determine which files already exist before installing, so we can
	// report accurate skip warnings (checking after install would always
	// find the files we just wrote).
	skipped, err := skippedSkills(opts.targetDir, opts.force)
	if err != nil {
		return err
	}

	installed, err := installSkills(opts.targetDir, opts.force)
	if err != nil {
		return err
	}

	c := opts.io.Color()
	for _, path := range skipped {
		fmt.Fprintf(opts.io.StdErr, "%s %s already exists. Use --force to overwrite.\n", c.WarnIcon(), path)
	}

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

// skippedSkills returns a list of skill file paths that already exist and
// would be skipped (only when force is false).
func skippedSkills(targetDir string, force bool) ([]string, error) {
	if force {
		return nil, nil
	}

	var skipped []string

	err := fs.WalkDir(skills.BundledSkills, "bundled", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel("bundled", path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(targetDir, relPath)
		if _, statErr := os.Stat(destPath); statErr == nil {
			skipped = append(skipped, destPath)
		}
		return nil
	})

	return skipped, err
}

// installSkills writes the bundled SKILL.md files to the target directory.
// If force is false, existing files are skipped.
// Returns a list of file paths that were written.
func installSkills(targetDir string, force bool) ([]string, error) {
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

		// Skip existing files unless --force is set
		if !force {
			if _, statErr := os.Stat(destPath); statErr == nil {
				return nil
			}
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
