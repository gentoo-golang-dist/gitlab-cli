// Package setup implements `glab orbit setup`, the guided onboarding for
// Orbit. It composes three primitives — the orbit status check, the
// skills install path, and the local binary install path — into a single
// command so users do not have to know about each one individually.
package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/binarymgr"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/local"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/registry"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// orbitSkillName is the entry in the curated remote registry that ships
// the Orbit agent skill. It matches the directory under .agents/skills/.
const orbitSkillName = "orbit"

// skillsRelDir mirrors the path used by `glab skills install`. Kept in
// sync intentionally — both commands write to the same convention.
var skillsRelDir = filepath.Join(".agents", "skills")

type options struct {
	io        *iostreams.IOStreams
	cfg       config.Config
	apiClient func(repoHost string) (*api.Client, error)

	// flags
	hostname  string
	yes       bool
	global    bool
	path      string
	upgrade   bool
	skipSkill bool
	skipLocal bool

	// resolved at complete-time
	skillTargetDir string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		cfg:       f.Config(),
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Guided setup for Orbit: verify access, install the skill, install the local CLI. (EXPERIMENTAL)",
		Long: heredoc.Doc(`
			Run a guided onboarding for the GitLab Knowledge Graph (Orbit):

			1. Verify that Orbit is reachable and enabled for your user.
			2. Install the Orbit agent skill so AI coding agents can discover it.
			3. Install the Orbit local CLI binary for indexing a local copy of the graph.

			Each step after the reachability check prompts for confirmation. Use
			`+"`--yes`"+` to accept every prompt, or `+"`--skip-skill`"+` / `+"`--skip-local`"+` to
			opt out of individual steps. Use `+"`--upgrade`"+` to re-fetch the skill and
			update the local binary in place.

			Exit codes match `+"`glab orbit remote`"+` — see `+"`glab orbit remote --help`"+`.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Interactive setup (prompts for each step)
			$ glab orbit setup

			# Non-interactive: accept every prompt
			$ glab orbit setup --yes

			# Install the skill globally (~/.agents/skills/) instead of in the current repo
			$ glab orbit setup --global

			# Re-fetch the skill and update the local CLI binary
			$ glab orbit setup --upgrade

			# Verify reachability only
			$ glab orbit setup --skip-skill --skip-local
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.complete(); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to verify. Defaults to the current repository's host or `gitlab.com`.")
	fl.BoolVarP(&opts.yes, "yes", "y", false, "Skip every confirmation prompt. (default false)")
	fl.BoolVarP(&opts.global, "global", "g", false, "Install the Orbit skill at user scope (`~/.agents/skills/`). (default false)")
	fl.StringVar(&opts.path, "path", "", "Install the Orbit skill to the directory at `<path>`.")
	fl.BoolVar(&opts.upgrade, "upgrade", false, "Re-fetch the skill and update the local CLI binary in place. (default false)")
	fl.BoolVar(&opts.skipSkill, "skip-skill", false, "Skip the agent-skill install step. (default false)")
	fl.BoolVar(&opts.skipLocal, "skip-local", false, "Skip the local CLI binary install step. (default false)")
	cmd.MarkFlagsMutuallyExclusive("global", "path")

	return cmd
}

func (o *options) complete() error {
	if o.skipSkill {
		return nil
	}
	if o.path != "" {
		o.skillTargetDir = o.path
		return nil
	}
	if o.global {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		o.skillTargetDir = filepath.Join(home, skillsRelDir)
		return nil
	}
	repoRoot, err := git.ToplevelDir()
	if err != nil {
		return fmt.Errorf("not in a Git repository. Use --global or --path to specify a target for the Orbit skill: %w", err)
	}
	o.skillTargetDir = filepath.Join(repoRoot, skillsRelDir)
	return nil
}

func (o *options) run(ctx context.Context) error {
	if err := o.verifyReachability(ctx); err != nil {
		return err
	}

	if err := o.installSkillStep(ctx); err != nil {
		return err
	}

	return o.installLocalStep(ctx)
}

// verifyReachability calls /api/v4/orbit/status and short-circuits the
// whole flow on any failure. Errors are routed through orbiterr.Translate
// so the user sees the same FF-off / 401 / 403 messaging they would get
// from `glab orbit remote status`.
func (o *options) verifyReachability(ctx context.Context) error {
	c := o.io.Color()
	o.io.LogInfof("%s Checking Orbit reachability...\n", c.DotWarnIcon())

	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	status, _, err := client.Lab().Orbit.GetStatus(nil, gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	host := o.hostname
	if host == "" {
		host = "the configured GitLab instance"
	}
	o.io.LogInfof("%s Orbit reachable at %s (status: %s, version: %s)\n",
		c.GreenCheck(), host, status.Status, status.Version)
	return nil
}

// installSkillStep fetches the orbit skill from the remote registry and
// writes its files into the resolved target directory. Idempotent: if
// the SKILL.md already exists and --upgrade is not set, the step is
// reported as a no-op rather than failing.
func (o *options) installSkillStep(ctx context.Context) error {
	if o.skipSkill {
		return nil
	}

	ok, err := o.confirm(ctx, fmt.Sprintf("Install the Orbit agent skill into %s?", o.skillTargetDir))
	if err != nil {
		return err
	}
	if !ok {
		c := o.io.Color()
		o.io.LogInfof("%s Skipped agent skill install.\n", c.DotWarnIcon())
		return nil
	}

	s, err := registry.Get(orbitSkillName)
	if err != nil {
		return fmt.Errorf("fetching Orbit skill from registry: %w", err)
	}

	alreadyInstalled, err := writeSkill(o.skillTargetDir, s, o.upgrade)
	if err != nil {
		return err
	}

	c := o.io.Color()
	skillDir := filepath.Join(o.skillTargetDir, s.Name)
	if alreadyInstalled {
		o.io.LogInfof("%s Orbit skill already installed at %s (use --upgrade to refresh).\n", c.DotWarnIcon(), skillDir)
	} else {
		o.io.LogInfof("%s Installed Orbit skill at %s\n", c.GreenCheck(), skillDir)
	}
	return nil
}

// writeSkill places each file under <targetDir>/<skill.Name>/. When
// force is false and the canonical SKILL.md already exists, the call is
// a no-op (matching `glab skills install` behavior); the bool is true
// in that case so the caller can surface the right message.
func writeSkill(targetDir string, s skill.Skill, force bool) (bool, error) {
	skillDir := filepath.Join(targetDir, s.Name)
	skillMDPath := filepath.Join(skillDir, skill.FileName)

	if _, statErr := os.Stat(skillMDPath); statErr == nil && !force {
		return true, nil
	}

	for rel, content := range s.Files {
		destPath := filepath.Join(skillDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return false, fmt.Errorf("creating directory for %s: %w", destPath, err)
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return false, fmt.Errorf("writing %s: %w", destPath, err)
		}
	}
	return false, nil
}

// installLocalStep installs (or, with --upgrade, updates) the Orbit
// local CLI binary using binarymgr.Runner — the same code path
// `glab orbit local --install` uses. Skipped automatically on
// platforms where Orbit local is not published.
func (o *options) installLocalStep(ctx context.Context) error {
	if o.skipLocal {
		return nil
	}

	spec := local.Spec()
	if _, err := spec.NormalizeArch(runtime.GOOS, runtime.GOARCH); err != nil {
		if errors.Is(err, binarymgr.ErrUnsupportedPlatform) {
			c := o.io.Color()
			o.io.LogInfof("%s Orbit local CLI is not available on %s/%s; skipping.\n",
				c.DotWarnIcon(), runtime.GOOS, runtime.GOARCH)
			return nil
		}
		return err
	}

	ok, err := o.confirm(ctx, "Install the Orbit local CLI binary?")
	if err != nil {
		return err
	}
	if !ok {
		c := o.io.Color()
		o.io.LogInfof("%s Skipped local CLI install.\n", c.DotWarnIcon())
		return nil
	}

	runner := &binarymgr.Runner{
		IO:      o.io,
		Cfg:     o.cfg,
		Spec:    spec,
		Manager: binarymgr.NewManager(o.io, spec),
		// Executor is intentionally nil — setup never executes the
		// binary, only installs/updates it.
		UpdateCommand: "orbit setup --upgrade",
		// Propagate the same auto-accept decision used by setup's own
		// prompts so binarymgr doesn't issue its own Download? prompt
		// in CI / Docker / agent subprocesses.
		Yes: o.autoAccept(),
	}

	if o.upgrade {
		return runner.HandleUpdate(ctx)
	}
	return runner.HandleInstall(ctx)
}

// confirm wraps io.Confirm so --yes / --upgrade can bypass prompts and
// non-TTY callers get a sane default (yes) instead of a hang.
func (o *options) confirm(ctx context.Context, prompt string) (bool, error) {
	if o.autoAccept() {
		return true, nil
	}
	result := true
	if err := o.io.Confirm(ctx, &result, prompt); err != nil {
		return false, err
	}
	return result, nil
}

// autoAccept reports whether every confirmation prompt — both setup's own
// and the binarymgr Runner's download prompt — should be skipped. The
// non-TTY branch matters most: agent subprocesses and CI runners hang on
// stdin without it.
func (o *options) autoAccept() bool {
	return o.yes || o.upgrade || !o.io.PromptEnabled()
}
