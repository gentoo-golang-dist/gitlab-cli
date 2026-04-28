package cmdutils

import (
	"context"
	"io"
	"os/exec"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

type Executor interface {
	// LookPath searches for an executable named file in the
	// directories named by the PATH environment variable.
	//
	// In production code forwards to exec.LookPath.
	LookPath(file string) (string, error)
	// Exec executes the given binary with args and env and sets up IO.
	//
	// In production code it sets up IO with the configured iostreams.
	Exec(ctx context.Context, name string, args []string, env []string) error
	// Exec executes the given binary with args and env and returns the combined stdout and stderr output.
	ExecWithCombinedOutput(ctx context.Context, name string, args []string, env []string) ([]byte, error)
	// Exec executes the given binary with args and env and configures the IOs with the given arguments.
	ExecWithIO(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type factoryExecutor struct {
	io *iostreams.IOStreams
}

func (f *factoryExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (f *factoryExecutor) Exec(ctx context.Context, name string, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = f.io.In
	cmd.Stdout = f.io.StdOut
	cmd.Stderr = f.io.StdErr
	return cmd.Run()
}

func (f *factoryExecutor) ExecWithCombinedOutput(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

func (f *factoryExecutor) ExecWithIO(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
