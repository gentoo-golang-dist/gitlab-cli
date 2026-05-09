//go:build unix

package local

import (
	"context"
	"os"
	"syscall"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// executeOrbit replaces the current process with the orbit binary on Unix
// so signal handling and terminal control work cleanly for any interactive
// flows the binary provides.
func executeOrbit(ctx context.Context, io *iostreams.IOStreams, binaryPath string, args []string) error {
	_, _ = ctx, io
	argv := append([]string{binaryPath}, args...)
	env := append(os.Environ(), "GITLAB_ORBIT_DISTRIBUTION=glab")
	return syscall.Exec(binaryPath, argv, env)
}
