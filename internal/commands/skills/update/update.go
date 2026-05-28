package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/installed"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/registry"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io        *iostreams.IOStreams
	requested string
	all       bool
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{io: f.IO()}

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update installed agent skills to the current shipped version. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Re-fetch installed agent skills from their source (bundled in this
			glab binary, or the curated remote registry) and overwrite the on-disk
			copy if it differs.

			By default, updates only the named skill in every well-known location
			it is installed (the current project's '.agents/skills/' and the
			user-scope '~/.agents/skills/'). Use --all to update every installed
			skill in those locations.

			Skills whose on-disk content already matches the source are left alone.
			Skills installed via 'glab skills install --path' are not considered —
			update only knows about the two well-known locations.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Update the bundled glab skill in every location it is installed
			glab skills update glab

			# Update every installed skill
			glab skills update --all
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	cmd.Flags().BoolVar(&opts.all, "all", false, "Update every installed skill. (default false)")

	return cmd
}

func (o *options) complete(args []string) {
	if len(args) == 1 {
		o.requested = args[0]
	}
}

func (o *options) validate() error {
	if o.requested == "" && !o.all {
		return cmdutils.WrapError(errors.New("missing argument"), "pass a skill name or use --all.")
	}
	if o.requested != "" && o.all {
		return cmdutils.WrapError(errors.New("flag conflict"), "cannot combine a skill name with --all.")
	}
	return nil
}

func (o *options) run() error {
	installedSkills, err := installed.Discover()
	if err != nil {
		return err
	}

	targets := installedSkills
	if !o.all {
		targets = filterByName(installedSkills, o.requested)
		if len(targets) == 0 {
			return cmdutils.WrapError(
				fmt.Errorf("not installed"),
				fmt.Sprintf("skill %q is not installed in any known location. Run 'glab skills install %s' first.", o.requested, o.requested),
			)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(o.io.StdErr, "No installed skills found.")
		return nil
	}

	c := o.io.Color()
	var updateErrs []error
	for _, ins := range targets {
		source, err := registry.Get(ins.Name)
		if err != nil {
			updateErrs = append(updateErrs, fmt.Errorf("%s: %w", ins.Name, err))
			continue
		}
		if skill.ContentHash(source.Files) == ins.Hash {
			o.io.LogInfof("%s %s (%s) already up to date.\n", c.GreenCheck(), ins.Name, ins.Scope)
			continue
		}
		if err := writeSkill(ins.Dir, source.Files); err != nil {
			updateErrs = append(updateErrs, fmt.Errorf("%s: %w", ins.Name, err))
			continue
		}
		o.io.LogInfof("%s Updated %s (%s).\n", c.GreenCheck(), ins.Name, ins.Scope)
	}
	return errors.Join(updateErrs...)
}

func filterByName(in []installed.Skill, name string) []installed.Skill {
	out := in[:0:0]
	for _, s := range in {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

// writeSkill mirrors install.installOne's write loop but doesn't gate
// on --force (update is explicit consent to overwrite).
func writeSkill(skillDir string, files map[string][]byte) error {
	for rel, content := range files {
		destPath := filepath.Join(skillDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", destPath, err)
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", destPath, err)
		}
	}
	return nil
}
