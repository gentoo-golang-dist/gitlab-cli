package orbit

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

func NewCmd(f cmdutils.Factory) *cobra.Command {
	orbitCmd := &cobra.Command{
		Use:   "orbit <command> [flags]",
		Short: "Graph Knowledge Graph tools",
		Long: heredoc.Doc(`
			Work with the Graph Knowledge Graph (gkg) to index and query
			your local repositories.

			The gkg binary must be installed and available on your PATH,
			or its path set via the GLAB_GKG_PATH environment variable.
		`),
	}

	orbitCmd.AddCommand(newIndexCmd(f))
	orbitCmd.AddCommand(newServerCmd(f))
	orbitCmd.AddCommand(newRemoveCmd(f))
	orbitCmd.AddCommand(newCleanCmd(f))

	return orbitCmd
}

// gkgPath returns the path to the gkg binary.
// GLAB_GKG_PATH env var overrides the PATH lookup.
func gkgPath() (string, error) {
	if p := os.Getenv("GLAB_GKG_PATH"); p != "" {
		return p, nil
	}
	return exec.LookPath("gkg")
}

// runGkg executes the gkg binary with the given arguments, streaming
// stdin/stdout/stderr directly to/from the user's terminal.
func runGkg(ctx context.Context, io *iostreams.IOStreams, gkgArgs []string) error {
	bin, err := gkgPath()
	if err != nil {
		return fmt.Errorf("gkg binary not found on PATH (install it or set GLAB_GKG_PATH): %w", err)
	}

	cmd := exec.CommandContext(ctx, bin, gkgArgs...)
	cmd.Stdin = io.In
	cmd.Stdout = io.StdOut
	cmd.Stderr = io.StdErr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return cmdutils.WrapError(err, "gkg execution failed")
	}

	return nil
}
