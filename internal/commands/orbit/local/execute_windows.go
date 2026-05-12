//go:build windows

package local

import (
	"context"
	"errors"
	"os"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// executeOrbit is a stub on Windows: the Orbit local CLI does not currently
// publish Windows binaries, and detectPlatform rejects this OS up front.
// This stub exists only so the package compiles on Windows.
func executeOrbit(ctx context.Context, io *iostreams.IOStreams, binaryPath string, args []string) error {
	_, _, _, _ = ctx, io, binaryPath, args
	_ = os.Stdin
	return errors.New("Orbit local CLI is not supported on Windows")
}
