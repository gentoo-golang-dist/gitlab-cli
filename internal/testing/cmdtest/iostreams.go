package cmdtest

import (
	"bytes"
	"context"
	io "io"
	"testing"

	tea "charm.land/bubbletea/v2"
	"git.sr.ht/~timofurrer/ugh"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// WithTestIOStreamsAsTTY sets stdin, stdout and stderr as TTY
// By default they are not treated as TTYs. This will overwrite the behavior
// for the three of them. If you only want to set a specific one,
// use iostreams.WithStdin, iostreams.WithStdout or iostreams.WithStderr.
func WithTestIOStreamsAsTTY(asTTY bool) iostreams.IOStreamsOption {
	return func(i *iostreams.IOStreams) {
		i.IsInTTY = asTTY
		i.IsaTTY = asTTY
		i.IsErrTTY = asTTY
	}
}

func TestIOStreamsWithConsole(t *testing.T, console *ugh.Console) (*iostreams.IOStreams, context.CancelFunc) {
	t.Helper()

	appInR, appInW := io.Pipe()
	appOutR, appOutW := io.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	wait := console.Start(ctx, appOutR, appInW)

	ios := iostreams.New(
		iostreams.WithStdin(appInR, true),
		iostreams.WithStdout(appOutW, true),
		iostreams.WithStderr(io.Discard, true),
		iostreams.WithProgramOptions(tea.WithWindowSize(ConsoleWidth, ConsoleHeight)),
	)

	cleanup := func() {
		appInW.Close()
		appOutW.Close()
		appOutR.Close()
		cancel()
		wait()
	}

	return ios, cleanup
}

func TestIOStreams(options ...iostreams.IOStreamsOption) (*iostreams.IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	opts := []iostreams.IOStreamsOption{
		iostreams.WithStdin(io.NopCloser(in), false),
		iostreams.WithStdout(out, false),
		iostreams.WithStderr(errOut, false),
	}
	opts = append(opts, options...)

	ios := iostreams.New(opts...)

	return ios, in, out, errOut
}
