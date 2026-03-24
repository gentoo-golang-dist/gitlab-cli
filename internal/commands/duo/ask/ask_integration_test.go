//go:build integration

package ask

import (
	"bytes"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"git.sr.ht/~timofurrer/ugh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/test"
)

func TestAskGit_Integration(t *testing.T) {
	glTestHost := test.GetHostOrSkip(t)
	t.Setenv("GITLAB_HOST", glTestHost)

	cfg, err := config.Init()
	require.NoError(t, err)

	appInR, appInW := io.Pipe()
	appOutR, appOutW := io.Pipe()
	rErr, wErr := io.Pipe()

	stdout := &bytes.Buffer{}
	mwOut := io.MultiWriter(appOutW, stdout)

	ios := iostreams.New(
		iostreams.WithStdin(appInR, false),
		iostreams.WithStdout(mwOut, false),
		iostreams.WithStderr(wErr, false),
		iostreams.WithProgramOptions(tea.WithWindowSize(cmdtest.ConsoleWidth, cmdtest.ConsoleHeight)),
	)

	f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})

	c := ugh.New(t, ugh.WithSize(cmdtest.ConsoleWidth, cmdtest.ConsoleHeight), ugh.WithTimeout(30*time.Second))
	c.Expect(ugh.Confirm(runCmdsQuestion)).
		Do(ugh.Reject)

	wait := c.Start(t.Context(), appOutR, appInW)

	cmd := NewCmdAsk(f)
	cli := "--git how to create a branch"

	_, err = cmdtest.ExecuteCommand(cmd, cli, nil, nil)
	require.NoError(t, err)

	appInW.Close()
	appOutR.Close()
	appOutW.Close()
	rErr.Close()
	wErr.Close()

	wait()

	out := stdout.String()

	for _, msg := range []string{"Commands", "Explanation", "git checkout"} {
		assert.Contains(t, out, msg)
	}
}
