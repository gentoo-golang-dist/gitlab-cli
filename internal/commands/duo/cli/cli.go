package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"

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
		Short: "Run the GitLab Duo CLI",
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
		`, "`"),
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
	// Check config to see if user has set auto-run
	cfg := o.Factory.Config()
	autoRun, _ := cfg.Get("", duoCLIAutoRunKey)

	if autoRun != "true" {
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
			if err := cfg.Set("", duoCLIAutoRunKey, "true"); err != nil {
				return fmt.Errorf("failed to save preference: %w", err)
			}
			if err := cfg.Write(); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}
		case "Yes":
			// Continue with one-time execution
		}
	}

	// Check if Node.js is installed and meets minimum version
	if err := o.checkNodeVersion(ctx); err != nil {
		return err
	}

	// Check if we should share the GitLab token with Duo CLI
	shareToken, _ := cfg.Get("", duoCLIShareTokenKey)
	var token string

	if shareToken != "true" && shareToken != "false" {
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
			shareToken = "false"
		case "Always":
			// Share and save preference
			if err := cfg.Set("", duoCLIShareTokenKey, "true"); err != nil {
				return fmt.Errorf("failed to save token sharing preference: %w", err)
			}
			if err := cfg.Write(); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}
			shareToken = "true"
		case "Never":
			// Don't share and save preference
			if err := cfg.Set("", duoCLIShareTokenKey, "false"); err != nil {
				return fmt.Errorf("failed to save token sharing preference: %w", err)
			}
			if err := cfg.Write(); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}
			shareToken = "false"
		case "Yes":
			// Share this time only
			shareToken = "true"
		}
	}

	// Get the token if user wants to share it
	if shareToken == "true" {
		// Get the default hostname
		hostname := o.Factory.DefaultHostname()

		// Try to get the token for this host
		token, _ = cfg.Get(hostname, "token")
	}

	// Execute the Duo CLI via npx
	// The Duo CLI is fully interactive and doesn't take command-line arguments
	duoCmd := exec.CommandContext(ctx, "npx", duoCLIPackage)

	// Restore original stdin for the Duo CLI
	// After huh prompts, we need to reconnect to the actual terminal
	// to ensure the Duo CLI gets clean stdin
	if o.IO.IsInTTY {
		tty, err := os.Open("/dev/tty")
		if err == nil {
			duoCmd.Stdin = tty
			defer tty.Close()
		} else {
			duoCmd.Stdin = o.IO.In
		}
	} else {
		duoCmd.Stdin = o.IO.In
	}

	duoCmd.Stdout = o.IO.StdOut
	duoCmd.Stderr = o.IO.StdErr

	// Pass the token as an environment variable if available
	if token != "" {
		duoCmd.Env = append(os.Environ(), "GITLAB_TOKEN="+token)
	}

	if err := duoCmd.Run(); err != nil {
		return cmdutils.WrapError(err, "failed to execute Duo CLI")
	}

	return nil
}

// checkNodeVersion verifies that Node.js is installed and meets the minimum version requirement
func (o *opts) checkNodeVersion(ctx context.Context) error {
	// Check if node is available
	nodeCmd := exec.CommandContext(ctx, nodeVersionCommand, "--version")
	output, err := nodeCmd.Output()
	if err != nil {
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
