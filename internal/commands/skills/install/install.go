package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// skillsRelDir is the conventional directory for agent skills,
// as defined by the Agent Skills specification (https://agentskills.io).
var skillsRelDir = filepath.Join(".agents", "skills")

type options struct {
	io        *iostreams.IOStreams
	global    bool
	path      string
	force     bool
	requested string
	targetDir string
}

func NewCmdInstall(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io: f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "install [name]",
		Short: "Install glab's bundled agent skills. (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Install bundled %[1]sSKILL.md%[1]s files to %[1]s.agents/skills/%[1]s, the
			cross-agent standard defined by the Agent Skills specification. This works
			with GitLab Duo Agent Platform, Claude Code, Codex, Gemini CLI, and any
			other compliant agent.

			By default, all bundled skills are installed. Pass a positional %[1]sname%[1]s
			argument to install a single skill. Run %[1]sglab skills list%[1]s to see what
			is available.

			Install scope:

			- By default, skills are installed for the current project, in %[1]s.agents/skills/%[1]s
			  at the root of the current Git repository.
			- Use %[1]s--global%[1]s to install skills for the current user, in
			  %[1]s~/.agents/skills/%[1]s.
			- Use %[1]s--path%[1]s to install skills to a custom directory. The path is resolved
			  relative to the current working directory, not the repository root.

			To overwrite existing skill files, use %[1]s--force%[1]s.
		`, "`") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Install all bundled skills in the current project (default)
			glab skills install

			# Install a single skill by name
			glab skills install glab

			# Install skills globally (user scope)
			glab skills install --global

			# Install skills to a custom directory
			glab skills install --path /path/to/skills

			# Overwrite existing skill files
			glab skills install --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}
			return opts.run()
		},
	}

	fl := cmd.Flags()
	fl.BoolVarP(&opts.global, "global", "g", false, "Install skills at user scope (~/.agents/skills/). (default false)")
	fl.StringVar(&opts.path, "path", "", "Install skills to the directory at <path>.")
	fl.BoolVarP(&opts.force, "force", "f", false, "Overwrite existing skill files. (default false)")
	cmd.MarkFlagsMutuallyExclusive("global", "path")

	return cmd
}

func (o *options) complete(args []string) error {
	if len(args) == 1 {
		o.requested = args[0]
	}

	if o.path != "" {
		o.targetDir = o.path
		return nil
	}

	if o.global {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		o.targetDir = filepath.Join(home, skillsRelDir)
		return nil
	}

	repoRoot, err := git.ToplevelDir()
	if err != nil {
		return fmt.Errorf("not in a Git repository. Use --global or --path to specify a target: %w", err)
	}
	o.targetDir = filepath.Join(repoRoot, skillsRelDir)
	return nil
}

func (o *options) run() error {
	skills, err := o.resolveSkills()
	if err != nil {
		return err
	}

	var errs []error
	for _, s := range skills {
		if err := o.installOne(s); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o *options) resolveSkills() ([]bundled.Skill, error) {
	if o.requested != "" {
		s, err := bundled.Get(o.requested)
		if err != nil {
			return nil, err
		}
		return []bundled.Skill{s}, nil
	}
	return bundled.All()
}

func (o *options) installOne(s bundled.Skill) error {
	destPath := filepath.Join(o.targetDir, s.Name, bundled.FileName)
	_, statErr := os.Stat(destPath)
	exists := statErr == nil

	c := o.io.Color()
	if exists && !o.force {
		o.io.LogErrorf("%s %s already exists. Use --force to overwrite.\n", c.WarnIcon(), destPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", destPath, err)
	}

	if err := os.WriteFile(destPath, s.Content, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", destPath, err)
	}

	if exists {
		o.io.LogInfof("%s Overwrote %s\n", c.GreenCheck(), destPath)
	} else {
		o.io.LogInfof("%s Installed %s\n", c.GreenCheck(), destPath)
	}
	return nil
}
