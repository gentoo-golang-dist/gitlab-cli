//go:build unix

package cli

import (
	"context"
	"os"
	"syscall"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// executeDuoCLI executes the Duo CLI binary using exec() on Unix systems.
// This replaces the current glab process with the Duo CLI process,
// providing better signal handling and terminal control for the interactive TUI.
func executeDuoCLI(ctx context.Context, io *iostreams.IOStreams, binaryPath string, args []string) error {
	// Context and io are unused here: syscall.Exec replaces the process
	// immediately, so cancellation and stream redirection don't apply.
	_, _ = ctx, io
	argv := append([]string{binaryPath}, args...)

	// Pass full environment to duo binary (PATH, HOME, LANG, etc).
	// syscall.Exec requires explicit env — there's no parent to inherit from
	// since we're replacing the process entirely, not creating a subprocess.
	env := append(os.Environ(), "GITLAB_DUO_DISTRIBUTION=glab")
	return syscall.Exec(binaryPath, argv, env)
}
