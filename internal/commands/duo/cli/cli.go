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
		Short: "Run the GitLab Duo CLI (EXPERIMENTAL)",
		Long: heredoc.Docf(`Run the GitLab Duo CLI.

		This command provides an AI-powered assistant for your development workflow.
		The Duo CLI binary is automatically downloaded and managed by glab.

		The binary is installed to glab's config directory (respects XDG spec and
		%[1]sGLAB_CONFIG_DIR%[1]s). Authentication is handled automatically via
		%[1]sglab auth credential-helper%[1]s with OAuth2 token refresh support.

		Ensure you are authenticated before running: %[1]sglab auth login%[1]s

		Configuration options:

		- %[1]sduo_cli_auto_run%[1]s: Skip run confirmation prompt
		- %[1]sduo_cli_auto_download%[1]s: Skip download confirmation prompt

		Note: All arguments and flags are passed through to the Duo CLI binary.
		Use %[1]s--update%[1]s to check for and install updates to the binary.
	`, "`") + text.ExperimentalString,
		Example: heredoc.Doc(`
		# Run GitLab Duo CLI
		$ glab duo cli

		# Show Duo CLI help
		$ glab duo cli --help

		# Run using the alias (glab duo defaults to cli)
		$ glab duo

		# Check for and install updates
		$ glab duo cli --update
	`),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --update flag manually since DisableFlagParsing is true
			if len(args) > 0 && args[0] == "--update" {
				opts.update = true
				return opts.run(cmd.Context())
			}

			// Convert --help/-h to help command for duo binary
			for i, arg := range args {
				if arg == "--help" || arg == "-h" {
					args[i] = "help"
					break
				}
			}

			opts.complete(args)
			return opts.run(cmd.Context())
		},
	}

	return cmd
}

// shouldForceUpdateCheck returns true if update checks should ignore the 24h delay.
func shouldForceUpdateCheck() bool {
	return os.Getenv("GLAB_DUO_CLI_CHECK_UPDATE") == "true"
}

// updateCheckResult contains the result of an update check.
type updateCheckResult struct {
	hasUpdate      bool
	currentVersion string
	latestVersion  string
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

	hasUpdate, latestVersion, newCheckTime, err := o.manager.CheckForUpdate(ctx, currentVersion, lastCheckTime, forceCheck)
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
		hasUpdate:      hasUpdate,
		currentVersion: currentVersion,
		latestVersion:  latestVersion,
	}, nil
}

func (o *options) complete(args []string) {
	o.args = args
}

func (o *options) run(ctx context.Context) error {
	if o.update {
		return o.handleUpdate(ctx)
	}

	installedVersion, _ := o.cfg.Get("", "duo_cli_binary_version")
	installedPath, _ := o.cfg.Get("", "duo_cli_binary_path")
	autoDownload, _ := o.cfg.Get("", "duo_cli_auto_download")

	info, err := o.manager.EnsureInstalled(ctx, installedVersion, installedPath, autoDownload)
	if err != nil {
		return err
	}

	if err := o.saveBinaryInfo(info); err != nil {
		color := o.io.Color()
		o.io.LogInfof("%s Failed to save binary metadata: %v\n", color.DotWarnIcon(), err)
	}

	o.checkForUpdates(ctx)

	if err := o.checkAutoRun(ctx); err != nil {
		return err
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
		info, err := o.manager.EnsureInstalled(ctx, installedVersion, installedPath, autoDownload)
		if err != nil {
			return err
		}
		return o.saveBinaryInfo(info)
	}

	if !result.hasUpdate {
		color := o.io.Color()
		o.io.LogInfof("%s You are already using the latest version (%s)\n", color.GreenCheck(), result.currentVersion)
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
	var confirm bool
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

	if result.hasUpdate {
		color := o.io.Color()
		o.io.LogInfof("\n%s New Duo CLI version available: %s → %s\n", color.DotWarnIcon(), result.currentVersion, result.latestVersion)
		o.io.LogInfof("Run 'glab duo cli --update' to upgrade\n")
	}
}
