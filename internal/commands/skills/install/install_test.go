//go:build !integration

package install

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdInstall_PathFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--path " + tmpDir)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.FileExists(t, filepath.Join(tmpDir, skillName, skillFile))
}

func TestNewCmdInstall_GlobalFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--global")

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.FileExists(t, filepath.Join(home, skillsRelDir, skillName, skillFile))
}

func TestNewCmdInstall_DefaultScopeOutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())

	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	_, err := exec("")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in a Git repository")
}

func TestNewCmdInstall_DefaultScopeInsideRepo(t *testing.T) {
	repoDir := git.InitGitRepo(t)
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("")

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.FileExists(t, filepath.Join(repoDir, skillsRelDir, skillName, skillFile))
}

func TestNewCmdInstall_GlobalAndPathMutuallyExclusive(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	_, err := exec("--global --path /tmp/skills")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group [global path] are set none of the others can be")
}

func TestNewCmdInstall_ForceOverwrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)

	// First install
	_, err := exec("--path " + tmpDir)
	require.NoError(t, err)

	// Force overwrite
	out, err := exec("--force --path " + tmpDir)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Overwrote")
}

func TestNewCmdInstall_SkipsWithoutForce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)

	// First install
	_, err := exec("--path " + tmpDir)
	require.NoError(t, err)

	// Second install without force
	out, err := exec("--path " + tmpDir)
	require.NoError(t, err)
	assert.Contains(t, out.Stderr(), "already exists. Use --force to overwrite")
}

func TestNewCmdInstall_FreshInstallNoWarnings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--path " + tmpDir)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.Empty(t, out.Stderr(), "expected no stderr output on fresh install")
}

func TestComplete(t *testing.T) {
	t.Parallel()

	t.Run("with --path", func(t *testing.T) {
		t.Parallel()

		o := &options{path: "/custom/path"}
		require.NoError(t, o.complete())
		assert.Equal(t, "/custom/path", o.targetDir)
	})

	t.Run("with --global", func(t *testing.T) {
		t.Parallel()

		o := &options{global: true}
		require.NoError(t, o.complete())
		assert.True(t, filepath.IsAbs(o.targetDir), "expected absolute path, got %s", o.targetDir)
		assert.True(t, strings.HasSuffix(o.targetDir, skillsRelDir))
	})
}

func TestBundledSkillContent(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, bundledSkillContent)

	text := string(bundledSkillContent)
	assert.Contains(t, text, "---")
	assert.Contains(t, text, "name: glab")
	assert.Contains(t, text, "description:")
}
