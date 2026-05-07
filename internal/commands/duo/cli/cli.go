package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/duo/cli/cliutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io      *iostreams.IOStreams
	cfg     config.Config
	manager *cliutils.BinaryManager
	update  bool
	install bool
	yes     bool
	args    []string // Arguments to pass through to Duo CLI
}

// NewCmd creates the `glab duo cli` command.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:      f.IO(),
		cfg:     f.Config(),
		manager: cliutils.NewBinaryManager(f.IO()),
	}

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

		Example: heredoc.Docf(`
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
			// Manually parse glab-owned flags since DisableFlagParsing is true.
			// Unrecognised args are collected and passed through to the duo binary.
			// --help/-h shows glab's help only when nothing has been collected for
			// pass-through yet; otherwise it belongs to a duo subcommand
			// (e.g. `glab duo cli run --help`).
			var remaining []string
			for _, arg := range args {
				switch arg {
				case "--update":
					opts.update = true
				case "--install":
					opts.install = true
				case "--yes", "-y":
					opts.yes = true
				case "--help", "-h":
					if len(remaining) == 0 {
						return cmd.Help()
					}
					remaining = append(remaining, arg)
				default:
					remaining = append(remaining, arg)
				}
			}

			if opts.install {
				return opts.handleInstall(cmd.Context())
			}

			opts.complete(remaining)
			return opts.run(cmd.Context())
		},
	}

	// Registered for documentation only — DisableFlagParsing means Cobra never
	// parses these; the RunE switch above handles them manually.
	fl := cmd.Flags()
	fl.BoolP("yes", "y", false, "Skip confirmation prompts. (default false)")
	fl.Bool("install", false, "Install the Duo CLI binary without running it. (default false)")
	fl.Bool("update", false, "Check for and install updates to the binary. (default false)")

	return cmd
}

// shouldForceUpdateCheck returns true if update checks should ignore the 24h delay.
func shouldForceUpdateCheck() bool {
	return os.Getenv("GLAB_DUO_CLI_CHECK_UPDATE") == "true"
}

// updateCheckResult contains the result of an update check.
type updateCheckResult struct {
	hasUpdate       bool
	currentVersion  string
	latestVersion   string
	newMajorVersion string // non-empty when a newer incompatible major is available
}

// performUpdateCheck checks for available updates and saves the check timestamp.
// Returns nil result (not error) if no version is installed.
// If forceCheck is true, ignores the 24h delay and checks immediately.
func (o *options) performUpdateCheck(ctx context.Context, forceCheck bool) (*updateCheckResult, error) {
	currentVersion, _ := o.cfg.Get("", "duo_cli_binary_version")
	if currentVersion == "" {
		return nil, nil
	}

	lastCheckStr, _ := o.cfg.Get("", "duo_cli_last_update_check")
	var lastCheckTime time.Time
	if lastCheckStr != "" {
		lastCheckTime, _ = time.Parse(time.RFC3339, lastCheckStr)
	}

	hasUpdate, latestVersion, newMajorVersion, newCheckTime, err := o.manager.CheckForUpdate(ctx, currentVersion, lastCheckTime, forceCheck)
	if err != nil {
		return nil, err
	}

	if !newCheckTime.IsZero() {
		if err := o.cfg.Set("", "duo_cli_last_update_check", newCheckTime.Format(time.RFC3339)); err != nil {
			color := o.io.Color()
			o.io.LogInfof("%s Failed to save update check time: %v\n", color.DotWarnIcon(), err)
		}
		if err := o.cfg.Write(); err != nil {
			color := o.io.Color()
			o.io.LogInfof("%s Failed to write config: %v\n", color.DotWarnIcon(), err)
		}
	}

	return &updateCheckResult{
		hasUpdate:       hasUpdate,
		currentVersion:  currentVersion,
		latestVersion:   latestVersion,
		newMajorVersion: newMajorVersion,
	}, nil
}

func (o *options) complete(args []string) {
	o.args = args
}

func (o *options) run(ctx context.Context) error {
	managedPath, err := cliutils.ManagedBinaryPath()
	if err != nil {
		return err
	}

	installedPath, _ := o.cfg.Get("", "duo_cli_binary_path")

	if installedPath != "" && installedPath != managedPath && o.update {
		color := o.io.Color()
		o.io.LogInfof("%s Updates are not applicable when using a custom binary path (%s).\n", color.DotWarnIcon(), installedPath)
		return nil
	}

	if o.update {
		return o.handleUpdate(ctx)
	}

	installedVersion, _ := o.cfg.Get("", "duo_cli_binary_version")
	autoDownload, _ := o.cfg.Get("", "duo_cli_auto_download")
	if o.yes {
		autoDownload = "true"
	}

	info, err := o.manager.EnsureInstalled(ctx, installedVersion, installedPath, autoDownload)
	if err != nil {
		return err
	}

	if info.Path == managedPath {
		if err := o.saveBinaryInfo(info); err != nil {
			color := o.io.Color()
			o.io.LogInfof("%s Failed to save binary metadata: %v\n", color.DotWarnIcon(), err)
		}
		o.checkForUpdates(ctx)
	} else {
		color := o.io.Color()
		o.io.LogInfof("%s Using custom Duo CLI binary: %s\n", color.DotWarnIcon(), info.Path)
	}

	if !o.yes {
		if err := o.checkAutoRun(ctx); err != nil {
			return err
		}
	}

	return o.executeDuoCLI(ctx, info.Path, o.args)
}

func (o *options) saveBinaryInfo(info *cliutils.BinaryInfo) error {
	if err := o.cfg.Set("", "duo_cli_binary_path", info.Path); err != nil {
		return err
	}
	if err := o.cfg.Set("", "duo_cli_binary_version", info.Version); err != nil {
		return err
	}
	if err := o.cfg.Set("", "duo_cli_binary_checksum", info.Checksum); err != nil {
		return err
	}
	return o.cfg.Write()
}

func (o *options) handleInstall(ctx context.Context) error {
	managedPath, err := cliutils.ManagedBinaryPath()
	if err != nil {
		return err
	}

	installedVersion, _ := o.cfg.Get("", "duo_cli_binary_version")
	installedPath, _ := o.cfg.Get("", "duo_cli_binary_path")
	autoDownload, _ := o.cfg.Get("", "duo_cli_auto_download")

	if o.yes {
		autoDownload = "true"
	}

	info, err := o.manager.EnsureInstalled(ctx, installedVersion, installedPath, autoDownload)
	if err != nil {
		return err
	}

	if info.Path != managedPath {
		color := o.io.Color()
		o.io.LogInfof("%s Using custom Duo CLI binary: %s\n", color.DotWarnIcon(), info.Path)
		return nil
	}

	if installedVersion != "" && info.Version == installedVersion {
		color := o.io.Color()
		o.io.LogInfof("%s Duo CLI version %s is already installed.\n", color.GreenCheck(), installedVersion)
		return nil
	}

	return o.saveBinaryInfo(info)
}

func (o *options) handleUpdate(ctx context.Context) error {
	result, err := o.performUpdateCheck(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if result == nil {
		o.io.LogInfo("No version installed, downloading latest...\n")
		installedVersion, _ := o.cfg.Get("", "duo_cli_binary_version")
		installedPath, _ := o.cfg.Get("", "duo_cli_binary_path")
		autoDownload, _ := o.cfg.Get("", "duo_cli_auto_download")
		if o.yes {
			autoDownload = "true"
		}
		info, err := o.manager.EnsureInstalled(ctx, installedVersion, installedPath, autoDownload)
		if err != nil {
			return err
		}
		return o.saveBinaryInfo(info)
	}

	if !result.hasUpdate {
		color := o.io.Color()
		o.io.LogInfof("%s You are already using the latest compatible version (%s)\n", color.GreenCheck(), result.currentVersion)
		if result.newMajorVersion != "" {
			o.io.LogInfof("%s Duo CLI %s is available but requires a newer version of glab.\n", color.DotWarnIcon(), result.newMajorVersion)
			o.io.LogInfof("Run 'glab check-update' to check for the latest glab version.\n")
		}
		return nil
	}

	o.io.LogInfof("Update available: %s → %s\n", result.currentVersion, result.latestVersion)

	info, err := o.manager.Update(ctx)
	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	if err := o.saveBinaryInfo(info); err != nil {
		color := o.io.Color()
		o.io.LogInfof("%s Failed to save binary metadata: %v\n", color.DotWarnIcon(), err)
	}

	color := o.io.Color()
	o.io.LogInfof("%s Successfully updated to version %s\n", color.GreenCheck(), result.latestVersion)
	return nil
}

func (o *options) checkAutoRun(ctx context.Context) error {
	autoRun, _ := o.cfg.Get("", "duo_cli_auto_run")

	if autoRun == "true" {
		return nil
	}

	// "false" means "don't auto-run", not "never run"
	confirm := true // Default to yes so users can press Enter to proceed
	if err := o.io.Confirm(ctx, &confirm, "Run the GitLab Duo CLI?"); err != nil {
		return err
	}

	if !confirm {
		return errors.New("execution cancelled")
	}

	var always bool
	if err := o.io.Confirm(ctx, &always, "Always run without prompting?"); err != nil {
		color := o.io.Color()
		o.io.LogInfof("%s Failed to get auto-run preference: %v\n", color.DotWarnIcon(), err)
		return nil
	}

	if always {
		if err := o.cfg.Set("", "duo_cli_auto_run", "true"); err != nil {
			color := o.io.Color()
			o.io.LogInfof("%s Failed to save preference: %v\n", color.DotWarnIcon(), err)
		}
		if err := o.cfg.Write(); err != nil {
			color := o.io.Color()
			o.io.LogInfof("%s Failed to write config: %v\n", color.DotWarnIcon(), err)
		}
	}

	return nil
}

// checkForUpdates checks for updates in the background (non-blocking).
// Silently fails on error - this is a non-critical background operation.
func (o *options) checkForUpdates(ctx context.Context) {
	result, err := o.performUpdateCheck(ctx, shouldForceUpdateCheck())
	if err != nil || result == nil {
		return
	}

	color := o.io.Color()
	if result.hasUpdate {
		o.io.LogInfof("\n%s New Duo CLI version available: %s → %s\n", color.DotWarnIcon(), result.currentVersion, result.latestVersion)
		o.io.LogInfof("Run 'glab duo cli --update' to update to the latest version\n")
	}
	if result.newMajorVersion != "" {
		o.io.LogInfof("\n%s Duo CLI %s is available but requires a newer version of glab.\n", color.DotWarnIcon(), result.newMajorVersion)
		o.io.LogInfof("Run 'glab check-update' to check for the latest glab version.\n")
	}
}
