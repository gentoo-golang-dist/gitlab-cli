package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	npxCommand          = "npx"
	duoCLIAutoRunKey    = "duo_cli_auto_run"
	duoCLIShareTokenKey = "duo_cli_share_token"
	nodeCheckTimeout    = 5 * time.Second
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

	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Run the GitLab Duo CLI (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Run the GitLab Duo CLI as a plugin to the GitLab CLI.

			This command executes the GitLab Duo CLI using npx, providing an interactive
			AI-powered assistant for your development workflow.

			Requirements:

			- Node.js version 22 or higher must be installed (includes npx)

			On first run, you'll be prompted to:

			1. Confirm running the Duo CLI (with option to "Always" skip this prompt)
			2. Choose whether to share your GitLab token with Duo CLI (with "Always" or "Never" options)

			The Duo CLI package (%[1]s@gitlab/duo-cli%[1]s) is automatically fetched via npx
			and cached for subsequent runs.

			Token Authentication:

			The command checks for authentication in the following priority order:

			1. %[1]sGITLAB_TOKEN%[1]s environment variable
			2. Existing Duo CLI authentication (%[1]s~/.gitlab/storage.json%[1]s)
			3. Sharing your configured glab token (via the %[1]s--gitlab-auth-token%[1]s flag)

			If environment token or existing Duo CLI auth is detected, you won't be prompted
			about token sharing. Otherwise, you can choose to share your glab token.

			Configuration:

			- %[1]sduo_cli_auto_run%[1]s: Set to "true" to skip the run confirmation prompt
			- %[1]sduo_cli_share_token%[1]s: Set to "true" or "false" to skip the token sharing prompt

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

	return cmd
}

func (o *opts) run(ctx context.Context, _ []string) error {
	// 1. Validate prerequisites
	if err := o.validatePrerequisites(ctx); err != nil {
		return err
	}

	// 2. Check if user wants to run Duo CLI
	if err := o.checkAutoRun(ctx); err != nil {
		return err
	}

	// 3. Determine authentication method and get token if needed
	authToken, err := o.resolveAuthentication(ctx)
	if err != nil {
		return err
	}

	// 4. Execute Duo CLI
	return o.executeDuoCLI(ctx, authToken)
}

// validatePrerequisites checks Node.js and npx availability
func (o *opts) validatePrerequisites(ctx context.Context) error {
	if err := o.checkNodeVersion(ctx); err != nil {
		return err
	}
	return o.checkNpxAvailable(ctx)
}

// checkAutoRun prompts user to confirm running Duo CLI (unless auto-run is enabled)
func (o *opts) checkAutoRun(ctx context.Context) error {
	cfg := o.Factory.Config()
	autoRunStr, _ := cfg.Get("", duoCLIAutoRunKey)

	if autoRunStr != "" {
		if autoRun, err := strconv.ParseBool(autoRunStr); err == nil && autoRun {
			return nil
		}
	}

	return o.promptAutoRun(ctx)
}

// promptAutoRun shows the "Run the GitLab Duo CLI?" prompt
func (o *opts) promptAutoRun(ctx context.Context) error {
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

	if err := o.IO.Run(ctx, selector); err != nil {
		if errors.Is(err, iostreams.ErrUserCancelled) {
			return cmdutils.SilentError
		}
		return err
	}

	return o.handleAutoRunChoice(choice)
}

// handleAutoRunChoice processes the user's auto-run choice
func (o *opts) handleAutoRunChoice(choice string) error {
	switch choice {
	case "No":
		return cmdutils.SilentError
	case "Always":
		if err := o.saveConfigValue(duoCLIAutoRunKey, "true", "preference"); err != nil {
			return err
		}
		fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will always run Duo CLI without prompting")
	}
	return nil
}

// resolveAuthentication determines which authentication method to use
// Returns the token to pass via --gitlab-auth-token flag (empty string if using env or Duo CLI auth)
func (o *opts) resolveAuthentication(ctx context.Context) (string, error) {
	// Priority 1: GITLAB_TOKEN environment variable
	if token := o.checkEnvironmentToken(); token != "" {
		return "", nil
	}

	// Priority 2: Existing Duo CLI authentication
	if o.checkDuoCLIAuth() {
		return "", nil
	}

	// Priority 3: Share glab token (with prompt)
	return o.resolveConfigToken(ctx)
}

// checkEnvironmentToken checks for GITLAB_TOKEN environment variable
func (o *opts) checkEnvironmentToken() string {
	envToken := os.Getenv("GITLAB_TOKEN")
	if envToken != "" {
		fmt.Fprintln(o.IO.StdErr, "ℹ Using GITLAB_TOKEN from environment")
	}
	return envToken
}

// checkDuoCLIAuth checks for existing Duo CLI authentication
func (o *opts) checkDuoCLIAuth() bool {
	if o.hasDuoCLIAuth() {
		fmt.Fprintln(o.IO.StdErr, "ℹ Using existing Duo CLI authentication")
		return true
	}
	return false
}

// resolveConfigToken handles the token sharing prompt and retrieves config token if needed
func (o *opts) resolveConfigToken(ctx context.Context) (string, error) {
	cfg := o.Factory.Config()
	shareTokenStr, _ := cfg.Get("", duoCLIShareTokenKey)

	var shouldShare bool
	var hasPreference bool

	// Check if user has already set their preference
	if shareTokenStr != "" {
		if parsed, err := strconv.ParseBool(shareTokenStr); err == nil {
			shouldShare = parsed
			hasPreference = true
		}
	}

	// If no preference set, prompt the user
	if !hasPreference {
		shareTokenBool, err := o.promptTokenSharing(ctx)
		if err != nil {
			return "", err
		}
		shouldShare = shareTokenBool
	}

	// If user doesn't want to share, return empty string
	if !shouldShare {
		return "", nil
	}

	// Get the token from config
	return o.getConfigToken()
}

// promptTokenSharing shows the "Share your GitLab token?" prompt
func (o *opts) promptTokenSharing(ctx context.Context) (bool, error) {
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

	if err := o.IO.Run(ctx, selector); err != nil {
		if errors.Is(err, iostreams.ErrUserCancelled) {
			return false, cmdutils.SilentError
		}
		return false, err
	}

	return o.handleTokenSharingChoice(choice)
}

// handleTokenSharingChoice processes the user's token sharing choice
func (o *opts) handleTokenSharingChoice(choice string) (bool, error) {
	switch choice {
	case "No":
		return false, nil
	case "Always":
		if err := o.saveConfigValue(duoCLIShareTokenKey, "true", "token sharing preference"); err != nil {
			return false, err
		}
		fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will always share GitLab token with Duo CLI")
		return true, nil
	case "Never":
		if err := o.saveConfigValue(duoCLIShareTokenKey, "false", "token sharing preference"); err != nil {
			return false, err
		}
		fmt.Fprintln(o.IO.StdErr, "✓ Preference saved: will never share GitLab token with Duo CLI")
		return false, nil
	case "Yes":
		return true, nil
	}

	return false, nil
}

// getConfigToken retrieves the GitLab token from config
func (o *opts) getConfigToken() (string, error) {
	cfg := o.Factory.Config()
	hostname := o.Factory.DefaultHostname()

	token, _ := cfg.Get(hostname, "token")
	// Return empty string if token not found or empty
	// Duo CLI will need to authenticate separately in this case
	return token, nil
}

// executeDuoCLI executes the Duo CLI command with the provided auth token
func (o *opts) executeDuoCLI(ctx context.Context, authToken string) error {
	// Build command
	duoCmd := o.buildCommand(ctx, authToken)

	// Set up I/O
	o.setupCommandIO(duoCmd)

	// Execute
	if err := duoCmd.Run(); err != nil {
		return cmdutils.WrapError(err, "failed to execute Duo CLI")
	}

	return nil
}

// buildCommand creates the exec.Cmd for running Duo CLI
func (o *opts) buildCommand(ctx context.Context, authToken string) *exec.Cmd {
	args := []string{duoCLIPackage}
	if authToken != "" {
		args = append(args, "--gitlab-auth-token", authToken)
	}

	cmd := exec.CommandContext(ctx, npxCommand, args...)
	cmd.Env = os.Environ()
	return cmd
}

// setupCommandIO configures stdin, stdout, and stderr for the command
func (o *opts) setupCommandIO(cmd *exec.Cmd) {
	cmd.Stdin = o.IO.In
	cmd.Stdout = o.IO.StdOut
	cmd.Stderr = o.IO.StdErr
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

// checkNpxAvailable verifies that npx is installed
func (o *opts) checkNpxAvailable(ctx context.Context) error {
	// Add a reasonable timeout for the npx check
	ctx, cancel := context.WithTimeout(ctx, nodeCheckTimeout)
	defer cancel()

	// Check if npx is available
	npxCmd := exec.CommandContext(ctx, npxCommand, "--version")
	_, err := npxCmd.Output()
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("checking npx availability timed out after %v", nodeCheckTimeout)
		}
		return o.handleMissingNpx()
	}

	return nil
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

// handleMissingNpx provides instructions when npx is not installed
func (o *opts) handleMissingNpx() error {
	return fmt.Errorf("npx is required but not installed. npx is typically installed with Node.js. Please install Node.js version %d or higher from https://nodejs.org/", minNodeVersion)
}

// handleMissingNode provides instructions when Node.js is not installed
func (o *opts) handleMissingNode() error {
	return fmt.Errorf("Node.js is required but not installed. Please install Node.js version %d or higher from https://nodejs.org/", minNodeVersion)
}

// handleOldNodeVersion provides instructions when Node.js version is too old
func (o *opts) handleOldNodeVersion(currentVersion int) error {
	return fmt.Errorf("Node.js version %d is installed, but version %d or higher is required. Please install Node.js version %d or higher from https://nodejs.org/", currentVersion, minNodeVersion, minNodeVersion)
}

// hasDuoCLIAuth checks if the Duo CLI has existing authentication configured
// by checking for the presence and validity of ~/.gitlab/storage.json
func (o *opts) hasDuoCLIAuth() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	storagePath := filepath.Join(home, ".gitlab", "storage.json")

	// Read the storage file
	data, err := os.ReadFile(storagePath)
	if err != nil {
		// File doesn't exist or can't be read
		return false
	}

	// Parse the JSON to check for authentication token
	var storage struct {
		DuoCLIConfig struct {
			GitLabAuthToken string `json:"gitlabAuthToken"`
		} `json:"duo-cli-config"`
	}

	if err := json.Unmarshal(data, &storage); err != nil {
		// Invalid JSON
		return false
	}

	// Only return true if there's actually a token configured
	return storage.DuoCLIConfig.GitLabAuthToken != ""
}
