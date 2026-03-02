//go:build windows

package cli

import (
	"context"
	"os"
	"os/exec"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/duo/cli/cliutils"
)

// executeDuoCLI executes the Duo CLI binary using subprocess on Windows.
// Windows doesn't have exec(), so we use a subprocess and exit with its exit code.
// This makes the exit behavior consistent with Unix (where exec() replaces the process).
func (o *options) executeDuoCLI(ctx context.Context, binaryPath string, args []string) error {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdin = o.io.In
	cmd.Stdout = o.io.StdOut
	cmd.Stderr = o.io.StdErr
	cmd.Env = cliutils.BuildDuoCLIEnv()

	err := cmd.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return cmdutils.WrapError(err, "failed to execute Duo CLI")
	}

	return nil
}
