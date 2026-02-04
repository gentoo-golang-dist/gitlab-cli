//go:build !integration

package skill

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestSkill_DynamicCommandList_ReplacesPlaceholder(t *testing.T) {
	ios, _, outBuf, _ := cmdtest.TestIOStreams()

	f := cmdtest.NewTestFactory(ios)

	rootCmd := &cobra.Command{
		Use: "glab",
	}

	aliasCmd := &cobra.Command{
		Use:   "alias",
		Short: "Create, list, and delete aliases",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(versionCmd)

	skillCmd := NewCmdSkill(f)
	rootCmd.AddCommand(skillCmd)

	rootCmd.SetArgs([]string{"skill"})
	rootCmd.SetOut(&bytes.Buffer{})
	err := rootCmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()

	assert.NotContains(t, output, "<% DYNAMIC_COMMAND_LIST %>")
	assert.Contains(t, output, "- `glab alias`")
	assert.Contains(t, output, "- `glab version`")
}

func TestSkill_OutputsSkillContent(t *testing.T) {
	exec := cmdtest.SetupCmdForTest(t, NewCmdSkill, false)

	out, err := exec("")
	require.NoError(t, err)

	output := out.OutBuf.String()

	assert.Contains(t, output, "name: glab")
	assert.Contains(t, output, "## Commands")
	assert.NotContains(t, output, "<% DYNAMIC_COMMAND_LIST %>")
}
