package cli

import (
	"context"
	"errors"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/binarymgr"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// duoMaxCompatibleMajorVersion caps Duo CLI auto-updates to this major
// version. Bump after validating compatibility against the new major.
const duoMaxCompatibleMajorVersion = 8

// Spec returns the binarymgr.Spec describing the Duo CLI binary. It is
// exported so tests and other consumers can introspect it.
func Spec() binarymgr.Spec {
	return binarymgr.Spec{
		DisplayName:        "GitLab Duo CLI",
		ProjectID:          "46519181",
		PackageName:        "duo-cli",
		ConfigPrefix:       "duo_cli",
		EnvVarPrefix:       "GLAB_DUO_CLI",
		MaxCompatibleMajor: duoMaxCompatibleMajorVersion,
		SupportedOS:        []string{"darwin", "linux", "windows"},
		NormalizeArch:      duoNormalizeArch,
		AssetName:          duoAssetName,
		InstalledName:      duoInstalledName,
		// Duo ships raw binaries — no extraction needed.
		Extract: nil,
	}
}

func duoNormalizeArch(goos, goarch string) (string, error) {
	switch goarch {
	case "amd64":
		if goos == "windows" {
			return "x64-baseline", nil
		}
		return "x64", nil
	case "arm64", "aarch64":
		return "arm64", nil
	}
	return "", binarymgr.ErrUnsupportedPlatform
}

func duoAssetName(goos, arch string) string {
	name := "duo-" + goos + "-" + arch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func duoInstalledName(goos string) string {
	if goos == "windows" {
		return "duo.exe"
	}
	return "duo"
}

// NewCmd creates the `glab duo cli` command.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	spec := Spec()
	cmd := &cobra.Command{
		Use:   "cli [command]",
		Short: "Run the GitLab Duo CLI (Beta)",
		Long: heredoc.Docf(`Run the GitLab Duo CLI.

Use the GitLab Duo CLI to bring the GitLab Duo Agent Platform to your terminal.
Ask GitLab Duo questions about your codebase and use it to autonomously perform actions
on your behalf.

When you use the GitLab Duo CLI in the GitLab CLI, %[1]sglab%[1]s handles
authentication for you automatically.
You only need to authenticate once.

Prerequisites:

- Use GitLab 18.11 or later.
- Run %[1]sglab auth login%[1]s to authenticate.
- Meet the [prerequisites for GitLab Duo Agent Platform](https://docs.gitlab.com/user/duo_agent_platform/#prerequisites).
- Turn on [beta and experimental features](https://docs.gitlab.com/user/duo_agent_platform/turn_on_off/#turn-on-beta-and-experimental-features).

Configuration options:

- %[1]sduo_cli_auto_run%[1]s: Skip the run confirmation prompt.
- %[1]sduo_cli_auto_download%[1]s: Skip the download confirmation prompt.

All other arguments and flags are passed through to the GitLab Duo CLI binary.

For more information, see the [GitLab Duo CLI documentation](https://docs.gitlab.com/user/gitlab_duo_cli/).
`, "`") + text.BetaString,
		Annotations: map[string]string{
			"help:environment": heredoc.Docf(`
				- %[1]sGLAB_DUO_CLI_BINARY_PATH%[1]s: Use a local binary instead of the managed one.
				  Skips download, version checks, and updates. Can also be set via the
				  %[1]sduo_cli_binary_path%[1]s configuration key.
				`, "`"),
		},
		Example: heredoc.Doc(`
			# Run the GitLab Duo CLI
			glab duo cli

			# Pass any command or flag through to the Duo CLI binary (for example: version, run, help)
			glab duo cli <command>

			# Show this help
			glab duo cli --help

			# Show Duo CLI help
			glab duo cli help

			# Run without prompts (for use in scripts and non-interactive environments)
			glab duo cli --yes

			# Install the Duo CLI binary
			glab duo cli --install

			# Install the Duo CLI binary without prompts
			glab duo cli --install --yes

			# Check for and install updates
			glab duo cli --update`),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := newRunner(f.IO(), f.Config(), spec)

			// DisableFlagParsing is on, so split glab-owned flags from
			// pass-through args manually. --help/-h shows glab's help only
			// when nothing has been collected for pass-through yet.
			var remaining []string
			for _, arg := range args {
				switch arg {
				case "--update":
					runner.Update = true
				case "--install":
					runner.Install = true
				case "--yes", "-y":
					runner.Yes = true
				case "--help", "-h":
					if len(remaining) == 0 {
						return cmd.Help()
					}
					remaining = append(remaining, arg)
				default:
					remaining = append(remaining, arg)
				}
			}
			runner.Args = remaining

			if runner.Install && runner.Update {
				return errors.New("the --install and --update flags are mutually exclusive")
			}

			if runner.Install {
				return runner.HandleInstall(cmd.Context())
			}
			return runner.Run(cmd.Context())
		},
	}

	// Registered for documentation only — DisableFlagParsing means Cobra
	// never parses these; the RunE switch above handles them manually.
	fl := cmd.Flags()
	fl.BoolP("yes", "y", false, "Skip confirmation prompts. (default false)")
	fl.Bool("install", false, "Install the Duo CLI binary without running it. (default false)")
	fl.Bool("update", false, "Check for and install updates to the binary. (default false)")

	return cmd
}

// newRunner builds a binarymgr.Runner for the Duo CLI. The Executor is
// platform-specific: syscall.Exec on Unix (in execute_unix.go), subprocess
// on Windows (in execute_windows.go). Each declares executeDuoCLI(io, ...).
func newRunner(io *iostreams.IOStreams, cfg config.Config, spec binarymgr.Spec) *binarymgr.Runner {
	return &binarymgr.Runner{
		IO:      io,
		Cfg:     cfg,
		Spec:    spec,
		Manager: binarymgr.NewManager(io, spec),
		Executor: func(ctx context.Context, binaryPath string, args []string) error {
			return executeDuoCLI(ctx, io, binaryPath, args)
		},
		UpdateCommand: "duo cli",
	}
}
