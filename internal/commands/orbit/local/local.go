package local

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

// Spec returns the binarymgr.Spec describing the Orbit local CLI binary.
// Orbit is pre-1.0 so MaxCompatibleMajor is intentionally unset (uncapped);
// add a cap when a 1.0+ release introduces a breaking surface.
func Spec() binarymgr.Spec {
	return binarymgr.Spec{
		DisplayName:   "Orbit local CLI",
		ProjectID:     "77960826",
		PackageName:   "orbit-local",
		ConfigPrefix:  "orbit_local",
		EnvVarPrefix:  "GLAB_ORBIT_LOCAL",
		SupportedOS:   []string{"darwin", "linux"},
		NormalizeArch: orbitNormalizeArch,
		AssetName:     orbitAssetName,
		InstalledName: orbitInstalledName,
		Extract:       binarymgr.TarGzExtractor("orbit"),
	}
}

func orbitNormalizeArch(goos, goarch string) (string, error) {
	if goos == "windows" {
		return "", binarymgr.ErrUnsupportedPlatform
	}
	switch goarch {
	case "amd64":
		return "x86_64", nil
	case "arm64", "aarch64":
		return "aarch64", nil
	}
	return "", binarymgr.ErrUnsupportedPlatform
}

// Tarballs are published as orbit-local-<os>-<arch>.tar.gz under the
// project's Generic Package Registry. The tarball contains a single
// `orbit` executable that we extract during install.
func orbitAssetName(goos, arch string) string {
	return "orbit-local-" + goos + "-" + arch + ".tar.gz"
}

func orbitInstalledName(string) string {
	return "orbit"
}

// NewCmd creates the `glab orbit local` command.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	spec := Spec()
	cmd := &cobra.Command{
		Use:   "local [command]",
		Short: "Run the Orbit local CLI (Experimental)",
		Long: heredoc.Docf(`Run the Orbit local CLI for the GitLab Knowledge Graph.

The Orbit local CLI lets you index and query a local copy of the GitLab Knowledge
Graph (product name: Orbit). When invoked through %[1]sglab%[1]s, the binary is
downloaded, verified, and kept up to date for you.

Configuration options:

- %[1]sorbit_local_auto_run%[1]s: Skip the run confirmation prompt.
- %[1]sorbit_local_auto_download%[1]s: Skip the download confirmation prompt.

All other arguments and flags are passed through to the Orbit local CLI binary.

For more information, see the [Orbit documentation](https://docs.gitlab.com/orbit/).
`, "`") + text.ExperimentalString,
		Annotations: map[string]string{
			"help:environment": heredoc.Docf(`
				- %[1]sGLAB_ORBIT_LOCAL_BINARY_PATH%[1]s: Use a local binary instead of the managed one.
				  Skips download, version checks, and updates. Can also be set via the
				  %[1]sorbit_local_binary_path%[1]s configuration key.
				`, "`"),
		},
		Example: heredoc.Doc(`
			# Run the Orbit local CLI
			glab orbit local

			# Pass any command or flag through to the orbit binary
			glab orbit local <command>

			# Show this help
			glab orbit local --help

			# Run without prompts (for use in scripts and non-interactive environments)
			glab orbit local --yes

			# Install the Orbit local CLI binary
			glab orbit local --install

			# Install the Orbit local CLI binary without prompts
			glab orbit local --install --yes

			# Check for and install updates
			glab orbit local --update`),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := newRunner(f.IO(), f.Config(), spec)

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
	fl.Bool("install", false, "Install the Orbit local CLI binary without running it. (default false)")
	fl.Bool("update", false, "Check for and install updates to the binary. (default false)")

	return cmd
}

func newRunner(io *iostreams.IOStreams, cfg config.Config, spec binarymgr.Spec) *binarymgr.Runner {
	return &binarymgr.Runner{
		IO:      io,
		Cfg:     cfg,
		Spec:    spec,
		Manager: binarymgr.NewManager(io, spec),
		Executor: func(ctx context.Context, binaryPath string, args []string) error {
			return executeOrbit(ctx, io, binaryPath, args)
		},
		UpdateCommand: "orbit local",
	}
}
