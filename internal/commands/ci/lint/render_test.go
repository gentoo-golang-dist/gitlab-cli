//go:build !integration

package lint

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/briandowns/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRenderLintResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		isRenderYAML bool
		result       *gitlab.ProjectLintResult
		wantStdout   string
		wantStderr   string
		wantErr      error
	}{
		{
			name: "valid summary writes success to stdout",
			result: &gitlab.ProjectLintResult{
				Valid: true,
			},
			wantStdout: "✓ CI/CD YAML is valid!\n",
		},
		{
			name: "invalid summary writes diagnostics to stderr",
			result: &gitlab.ProjectLintResult{
				Valid:  false,
				Errors: []string{"invalid yaml"},
			},
			wantStderr: testdataPath(".gitlab-ci.yaml") + " is invalid.\n1 invalid yaml\n",
			wantErr:    cmdutils.SilentError,
		},
		{
			name:         "valid render yaml keeps stdout machine clean",
			isRenderYAML: true,
			result: &gitlab.ProjectLintResult{
				Valid:      true,
				MergedYaml: "merged-yaml\n",
			},
			wantStdout: "merged-yaml\n",
		},
		{
			name:         "invalid render yaml writes yaml to stdout and diagnostics to stderr",
			isRenderYAML: true,
			result: &gitlab.ProjectLintResult{
				Valid:      false,
				Errors:     []string{"invalid yaml"},
				MergedYaml: "merged-yaml\n",
			},
			wantStdout: "merged-yaml\n",
			wantStderr: testdataPath(".gitlab-ci.yaml") + " is invalid.\n1 invalid yaml\n",
			wantErr:    cmdutils.SilentError,
		},
		{
			name:         "valid render yaml with empty merged output stays silent",
			isRenderYAML: true,
			result: &gitlab.ProjectLintResult{
				Valid: true,
			},
		},
		{
			name:         "invalid render yaml with empty merged output only writes diagnostics",
			isRenderYAML: true,
			result: &gitlab.ProjectLintResult{
				Valid:  false,
				Errors: []string{"invalid yaml"},
			},
			wantStderr: testdataPath(".gitlab-ci.yaml") + " is invalid.\n1 invalid yaml\n",
			wantErr:    cmdutils.SilentError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ios, _, stdout, stderr := cmdtest.TestIOStreams()
			err := renderLintResult(&options{
				io:           ios,
				path:         testdataPath(".gitlab-ci.yaml"),
				isRenderYAML: tt.isRenderYAML,
			}, tt.result)

			if tt.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantStdout, stdout.String())
			assert.Equal(t, tt.wantStderr, stderr.String())
		})
	}
}

func TestRenderLintResult_Errors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		io     *iostreams.IOStreams
		result *gitlab.ProjectLintResult
	}{
		{name: "stdout write failure is returned", io: newTestIO(errWriter{}, false, &bytes.Buffer{}, false), result: &gitlab.ProjectLintResult{Valid: true}},
		{name: "stderr write failure is returned", io: newTestIO(&bytes.Buffer{}, false, errWriter{}, false), result: &gitlab.ProjectLintResult{Valid: false, Errors: []string{"invalid yaml"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := renderLintResult(&options{io: tt.io, path: testdataPath(".gitlab-ci.yaml")}, tt.result)
			require.Error(t, err)
			assert.ErrorIs(t, err, io.ErrClosedPipe)
		})
	}
}

func TestRenderLintResult_ColorsStderrWhenStdoutIsPiped(t *testing.T) {
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	t.Setenv("COLOR_ENABLED", "")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ios := newTestIO(stdout, false, stderr, true)

	err := renderLintResult(&options{
		io:           ios,
		path:         testdataPath(".gitlab-ci.yaml"),
		isRenderYAML: true,
	}, &gitlab.ProjectLintResult{
		Valid:      false,
		Errors:     []string{"invalid yaml"},
		MergedYaml: "merged-yaml\n",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, cmdutils.SilentError))

	assert.Equal(t, "merged-yaml\n", stdout.String())
	assert.Contains(t, stderr.String(), "\x1b[31m")
	assert.Contains(t, stderr.String(), testdataPath(".gitlab-ci.yaml")+" is invalid.")
}

func TestStderrColor(t *testing.T) {
	t.Run("enabled and colorized for tty stderr", func(t *testing.T) {
		require.NoError(t, os.Unsetenv("NO_COLOR"))
		t.Setenv("COLOR_ENABLED", "")

		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		require.True(t, stderrColorEnabled(ios))
		require.Equal(t, "\x1b[31mboom\x1b[m", colorizeStderr(ios, "boom", "red"))
		require.Equal(t, "boom", colorizeStderr(ios, "boom", "blue"))
	})

	t.Run("disabled when stderr is not a tty", func(t *testing.T) {
		ios, _, _, _ := cmdtest.TestIOStreams()
		ios.IsErrTTY = false
		require.False(t, stderrColorEnabled(ios))
	})

	t.Run("no_color can disable and color_enabled can override", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("COLOR_ENABLED", "")

		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		require.False(t, stderrColorEnabled(ios))

		t.Setenv("COLOR_ENABLED", "1")
		require.True(t, stderrColorEnabled(ios))
	})
}

func TestIsSpinnable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		isRenderYAML bool
		stdoutTTY    bool
		stderrTTY    bool
		want         bool
	}{
		{
			name:      "stdout and stderr tty",
			stdoutTTY: true,
			stderrTTY: true,
			want:      true,
		},
		{
			name:         "stdout piped stderr tty while render yaml",
			isRenderYAML: true,
			stdoutTTY:    false,
			stderrTTY:    true,
			want:         true,
		},
		{
			name:         "stdout piped stderr not tty while render yaml",
			isRenderYAML: true,
			stdoutTTY:    false,
			stderrTTY:    false,
			want:         false,
		},
		{
			name:      "summary mode with piped stdout is not spinnable",
			stdoutTTY: false,
			stderrTTY: true,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, _ := cmdtest.TestIOStreams()
			ios.IsaTTY = tt.stdoutTTY
			ios.IsErrTTY = tt.stderrTTY

			assert.Equal(t, tt.want, isSpinnable(&options{
				io:           ios,
				isRenderYAML: tt.isRenderYAML,
			}))
		})
	}
}

func TestStartSpinner(t *testing.T) {
	t.Parallel()

	t.Run("non spinnable writes progress to stderr", func(t *testing.T) {
		t.Parallel()

		ios, _, _, stderr := cmdtest.TestIOStreams()
		s := startSpinner(&options{io: ios}, "validating...")
		require.Nil(t, s)
		assert.Equal(t, "validating...\n", stderr.String())
	})

	t.Run("spinnable returns spinner instance", func(t *testing.T) {
		t.Parallel()

		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		s := startSpinner(&options{io: ios}, "validating...")
		require.NotNil(t, s)
		assert.Equal(t, " validating...", s.Suffix)
		stopSpinner(s)
	})
}

func TestStopSpinner(t *testing.T) {
	t.Parallel()

	stopSpinner(nil)

	s := spinner.New(spinner.CharSets[9], 100)
	stopSpinner(s)
}
