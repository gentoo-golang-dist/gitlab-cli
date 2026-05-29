//go:build windows

package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// errorBadExeFormat is the Windows ERROR_BAD_EXE_FORMAT system error code,
// returned when the OS cannot run a binary (e.g. an x86_64 image on ARM64
// without emulation). Comparing the code is locale-independent, unlike the
// localized error string.
const errorBadExeFormat = syscall.Errno(193)

// executeOrbit runs the orbit binary as a subprocess on Windows. Windows has
// no exec() equivalent, so we shell out and exit with the child's status to
// keep behavior close to the Unix syscall.Exec path.
//
// Upstream only ships x86_64. On ARM64 Windows the OS runs the binary under
// x64 emulation (Windows 11). If emulation is unavailable (older Windows
// builds, locked-down systems), the OS returns ERROR_BAD_EXE_FORMAT; we
// surface a hint pointing at that as the likely cause.
func executeOrbit(ctx context.Context, io *iostreams.IOStreams, binaryPath string, args []string) error {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdin = io.In
	cmd.Stdout = io.StdOut
	cmd.Stderr = io.StdErr
	cmd.Env = append(os.Environ(), "GITLAB_ORBIT_DISTRIBUTION=glab")

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Intentional: mirror the Unix syscall.Exec path by exiting with the
			// child's status. This skips deferred cleanup further up the stack;
			// do not "fix" it by returning the error instead.
			os.Exit(exitErr.ExitCode())
		}
		return wrapExecError(err)
	}
	return nil
}

// wrapExecError annotates Windows-specific exec failures with actionable
// guidance. ERROR_BAD_EXE_FORMAT on ARM64 typically means x64 emulation is
// not available; on x64 it usually means the file is corrupt.
func wrapExecError(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == errorBadExeFormat {
		if runtime.GOARCH == "arm64" {
			return cmdutils.WrapError(err, fmt.Sprintf(
				"failed to execute Orbit local CLI: the x86_64 binary could not run on ARM64 Windows. "+
					"This usually means x64 emulation is not enabled. Upgrade to Windows 11 or enable x64 emulation, "+
					"then retry %q",
				"glab orbit local"))
		}
		return cmdutils.WrapError(err, "failed to execute Orbit local CLI: the binary appears to be corrupted. Run `glab orbit local --update` to reinstall")
	}
	return cmdutils.WrapError(err, "failed to execute Orbit local CLI")
}
