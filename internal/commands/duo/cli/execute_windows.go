//go:build windows

package cli

import (
	"context"
	"os"
	"os/exec"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// executeDuoCLI executes the Duo CLI binary using subprocess on Windows.
// Windows doesn't have exec(), so we use a subprocess and exit with its exit code.
// This makes the exit behavior consistent with Unix (where exec() replaces the process).
func executeDuoCLI(ctx context.Context, io *iostreams.IOStreams, binaryPath string, args []string) error {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdin = io.In
	cmd.Stdout = io.StdOut
	cmd.Stderr = io.StdErr
	cmd.Env = append(os.Environ(), "GITLAB_DUO_DISTRIBUTION=glab")

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return cmdutils.WrapError(err, "failed to execute Duo CLI")
	}
	return nil
}
