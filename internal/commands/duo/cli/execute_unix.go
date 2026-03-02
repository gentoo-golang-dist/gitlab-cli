//go:build unix

package cli

import (
	"context"
	"syscall"

	"gitlab.com/gitlab-org/cli/internal/commands/duo/cli/cliutils"
)

// executeDuoCLI executes the Duo CLI binary using exec() on Unix systems.
// This replaces the current glab process with the Duo CLI process,
// providing better signal handling and terminal control for the interactive TUI.
func (o *options) executeDuoCLI(ctx context.Context, binaryPath string, args []string) error {
	// Context is unused because syscall.Exec doesn't support cancellation.
	// Process replacement happens immediately and cannot be cancelled.
	argv := append([]string{binaryPath}, args...)

	// Pass full environment to duo binary (PATH, HOME, LANG, etc).
	// syscall.Exec requires explicit env - there's no parent to inherit from
	// since we're replacing the process entirely, not creating a subprocess.
	return syscall.Exec(binaryPath, argv, cliutils.BuildDuoCLIEnv())
}
