//go:build !integration

package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func setupAddCmd(f cmdutils.Factory) *cobra.Command {
	return NewCmdAdd(f, func(root *cobra.Command) string {
		return "test skill content"
	})
}

func TestAdd_WritesToNewFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "SKILL.md")

	exec := cmdtest.SetupCmdForTest(t, setupAddCmd, false)

	out, err := exec(path)
	require.NoError(t, err)

	assert.Contains(t, out.OutBuf.String(), "Wrote skill to")
	assert.Contains(t, out.OutBuf.String(), path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test skill content", string(content))
}

func TestAdd_CreatesDirectories(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "nested", "dirs", "SKILL.md")

	exec := cmdtest.SetupCmdForTest(t, setupAddCmd, false)

	out, err := exec(path)
	require.NoError(t, err)

	assert.Contains(t, out.OutBuf.String(), "Wrote skill to")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test skill content", string(content))
}

func TestAdd_ReplacesWithForce(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "SKILL.md")

	err := os.WriteFile(path, []byte("existing content"), 0o644)
	require.NoError(t, err)

	exec := cmdtest.SetupCmdForTest(t, setupAddCmd, false)

	out, err := exec(path + " --force")
	require.NoError(t, err)

	assert.Contains(t, out.OutBuf.String(), "Wrote skill to")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test skill content", string(content))
}
