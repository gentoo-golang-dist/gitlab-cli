//go:build !integration

package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "")
}

func TestRootVersion(t *testing.T) {
	ios, _, stdout, _ := cmdtest.TestIOStreams()
	rootCmd := NewCmdRoot(cmdutils.NewFactory(ios, false, config.NewBlankConfig(), api.BuildInfo{Version: "v1.0.0", Commit: "abcdefgh"}))
	rootCmd.SetOut(stdout)
	assert.Nil(t, rootCmd.Flag("version").Value.Set("true"))
	assert.Nil(t, rootCmd.Execute())

	assert.Equal(t, "glab 1.0.0 (abcdefgh)\n", stdout.String())
}

func TestRootNoArg(t *testing.T) {
	var buf bytes.Buffer
	ios, _, _, _ := cmdtest.TestIOStreams()
	rootCmd := NewCmdRoot(cmdutils.NewFactory(ios, false, config.NewBlankConfig(), api.BuildInfo{Version: "v1.0.0", Commit: "abcdefgh"}))
	rootCmd.SetOut(&buf)
	assert.Nil(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "GLab is an open source GitLab CLI tool that brings GitLab to your command line.\n")
	assert.Contains(t, buf.String(), `USAGE
  glab <command> <subcommand> [flags]

CORE COMMANDS`)
}
