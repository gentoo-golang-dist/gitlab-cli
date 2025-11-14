package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/text"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

const (
	duoCLIPackage       = "@gitlab/duo-cli"
	minNodeVersion      = 22
	nodeVersionCommand  = "node"
	duoCLIAutoRunKey    = "duo_cli_auto_run"
	duoCLIShareTokenKey = "duo_cli_share_token"
	nodeCheckTimeout    = 5 * time.Second

	configValueTrue  = "true"
	configValueFalse = "false"
)

type opts struct {
	Factory cmdutils.Factory
	IO      *iostreams.IOStreams
}

func NewCmdCli(f cmdutils.Factory) *cobra.Command {
	opts := &opts{
		Factory: f,
		IO:      f.IO(),
	}

	duoCliCmd := &cobra.Command{
		Use:   "cli",
		Short: "Run the GitLab Duo CLI (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Run the GitLab Duo CLI as a plugin to the GitLab CLI.

			This command executes the GitLab Duo CLI using npx, providing an interactive
			AI-powered assistant for your development workflow.

			Requirements:

			- Node.js version 22 or higher must be installed

			On first run, you'll be prompted to:

			1. Confirm running the Duo CLI (with option to "Always" skip this prompt)
			2. Choose whether to share your GitLab token with Duo CLI (with "Always" or "Never" options)

			The Duo CLI package (%[1]s@gitlab/duo-cli%[1]s) is automatically fetched via npx
			and cached for subsequent runs.

			Configuration:

			- duo_cli_auto_run: Set to "true" to skip the run confirmation prompt
			- duo_cli_share_token: Set to "true" or "false" to skip the token sharing prompt

			You can reset these preferences with:

			- %[1]sglab config set duo_cli_auto_run false%[1]s
			- %[1]sglab config set duo_cli_share_token false%[1]s
		`, "`") + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab duo cli
			> Starts an interactive session with GitLab Duo CLI
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run(cmd.Context(), args)
		},
	}

	return duoCliCmd
}

func (o *opts) run(ctx context.Context, _ []string) error {
	// Check if Node.js is installed and meets minimum version first
	// This is fast and doesn't require user input, so fail early if needed
	if err := o.checkNodeVersion(ctx); err != nil {
		return err
	}

	// Check config to see if user has set auto-run
	cfg := o.Factory.Config()
	autoRun, _ := cfg.Get("", duoCLIAutoRunKey)

	if autoRun != configValueTrue {
		// Ask permission with three options: Yes, No, Always
		var choice string

		selector := huh.NewSelect[string]().
			Title("Run the GitLab Duo CLI?").
			Description("Requires Node.js 22+, will download via npx on first run").
			Options(
				huh.NewOption("Yes", "Yes"),
				huh.NewOption("No", "No"),
				huh.NewOption("Always", "Always"),
			).
			Value(&choice)

		err := o.IO.Run(ctx, selector)
		if err != nil {
			// iostreams.Run already prints "Cancelled." for user cancellation
			if errors.Is(err, iostreams.ErrUserCancelled) {
				return cmdutils.SilentError
			}
			return err
		}

		switch choice {
		case "No":
			return cmdutils.SilentError
		case "Always":
			// Save to config so we don't prompt again
			if err := o.saveConfigValue(duoCLIAutoRunKey, configValueTrue, "preference"); err != nil {
				return err
			}
			fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will always run Duo CLI without prompting")
		case "Yes":
			// Continue with one-time execution
		}
	}

	// Check if we should share the GitLab token with Duo CLI
	shareToken, _ := cfg.Get("", duoCLIShareTokenKey)
	var token string

	if shareToken != configValueTrue && shareToken != configValueFalse {
		// Ask user if they want to share their GitLab token
		var choice string

		selector := huh.NewSelect[string]().
			Title("Share your GitLab token with Duo CLI?").
			Description("This allows Duo CLI to authenticate automatically without separate login").
			Options(
				huh.NewOption("Yes", "Yes"),
				huh.NewOption("No", "No"),
				huh.NewOption("Always", "Always"),
				huh.NewOption("Never", "Never"),
			).
			Value(&choice)

		err := o.IO.Run(ctx, selector)
		if err != nil {
			// iostreams.Run already prints "Cancelled." for user cancellation
			if errors.Is(err, iostreams.ErrUserCancelled) {
				return cmdutils.SilentError
			}
			return err
		}

		switch choice {
		case "No":
			// Don't share this time, don't save preference
			shareToken = configValueFalse
		case "Always":
			// Share and save preference
			if err := o.saveConfigValue(duoCLIShareTokenKey, configValueTrue, "token sharing preference"); err != nil {
				return err
			}
			fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will always share GitLab token with Duo CLI")
			shareToken = configValueTrue
		case "Never":
			// Don't share and save preference
			if err := o.saveConfigValue(duoCLIShareTokenKey, configValueFalse, "token sharing preference"); err != nil {
				return err
			}
			fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will never share GitLab token with Duo CLI")
			shareToken = configValueFalse
		case "Yes":
			// Share this time only
			shareToken = configValueTrue
		}
	}

	// Get the token if user wants to share it
	if shareToken == configValueTrue {
		// Get the default hostname
		hostname := o.Factory.DefaultHostname()

		// Try to get the token for this host
		var err error
		token, err = cfg.Get(hostname, "token")
		if err != nil || token == "" {
			// Token not found or empty - Duo CLI will need to authenticate separately
			// This is not an error, just means no token to share
			token = ""
		}
	}

	// Execute the Duo CLI via npx
	// The Duo CLI is fully interactive and doesn't take command-line arguments
	duoCmd := exec.CommandContext(ctx, "npx", duoCLIPackage)

	// Restore stdin for the Duo CLI
	// After huh prompts, we need to reconnect to the actual terminal
	// to ensure the Duo CLI gets clean stdin
	if o.IO.IsInTTY {
		tty, err := o.openTerminal()
		if err != nil {
			// Fall back to standard input
			duoCmd.Stdin = o.IO.In
		} else {
			duoCmd.Stdin = tty
			defer tty.Close()
		}
	} else {
		duoCmd.Stdin = o.IO.In
	}

	duoCmd.Stdout = o.IO.StdOut
	duoCmd.Stderr = o.IO.StdErr

	// Pass the token as an environment variable if available
	if token != "" {
		duoCmd.Env = o.buildEnvironment(token)
	}

	if err := duoCmd.Run(); err != nil {
		return cmdutils.WrapError(err, "failed to execute Duo CLI")
	}

	return nil
}

// saveConfigValue saves a configuration value and writes the config file
func (o *opts) saveConfigValue(key, value, description string) error {
	cfg := o.Factory.Config()
	if err := cfg.Set("", key, value); err != nil {
		return fmt.Errorf("failed to save %s: %w", description, err)
	}
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// openTerminal attempts to open the controlling terminal for stdin
func (o *opts) openTerminal() (*os.File, error) {
	// On Windows, /dev/tty doesn't exist
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("terminal redirection not supported on Windows")
	}

	// On Unix-like systems, use /dev/tty
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/tty: %w", err)
	}

	return tty, nil
}

// buildEnvironment creates the environment for the Duo CLI subprocess
// It filters out any existing GITLAB_TOKEN and adds the new one
func (o *opts) buildEnvironment(token string) []string {
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env)+1)

	// Filter out existing GITLAB_TOKEN
	for _, e := range env {
		if !strings.HasPrefix(e, "GITLAB_TOKEN=") {
			filteredEnv = append(filteredEnv, e)
		}
	}

	// Add the new token
	filteredEnv = append(filteredEnv, "GITLAB_TOKEN="+token)
	return filteredEnv
}

// checkNodeVersion verifies that Node.js is installed and meets the minimum version requirement
func (o *opts) checkNodeVersion(ctx context.Context) error {
	// Add a reasonable timeout for the node version check
	ctx, cancel := context.WithTimeout(ctx, nodeCheckTimeout)
	defer cancel()

	// Check if node is available
	nodeCmd := exec.CommandContext(ctx, nodeVersionCommand, "--version")
	output, err := nodeCmd.Output()
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("checking Node.js version timed out after %v", nodeCheckTimeout)
		}
		return o.handleMissingNode()
	}

	// Parse version (format: v22.0.0)
	versionStr := strings.TrimSpace(string(output))
	versionStr = strings.TrimPrefix(versionStr, "v")

	// Extract major version
	parts := strings.Split(versionStr, ".")
	if len(parts) == 0 {
		return fmt.Errorf("failed to parse Node.js version: %s", versionStr)
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("failed to parse Node.js major version: %w", err)
	}

	if majorVersion < minNodeVersion {
		return o.handleOldNodeVersion(majorVersion)
	}

	return nil
}

// handleMissingNode provides instructions when Node.js is not installed
func (o *opts) handleMissingNode() error {
	return fmt.Errorf("Node.js is required but not installed. Please install Node.js version %d or higher from https://nodejs.org/", minNodeVersion)
}

// handleOldNodeVersion provides instructions when Node.js version is too old
func (o *opts) handleOldNodeVersion(currentVersion int) error {
	return fmt.Errorf("Node.js version %d is installed, but version %d or higher is required. Please upgrade Node.js from https://nodejs.org/", currentVersion, minNodeVersion)
}
