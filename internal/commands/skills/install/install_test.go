//go:build !integration

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
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
	assert.FileExists(t, filepath.Join(tmpDir, "glab", bundled.FileName))
	// Default install should only ship the core `glab` skill; other
	// bundled skills are opt-in by name to avoid context-window
	// pollution. Guard against regressing back to "install all".
	assert.NoFileExists(t, filepath.Join(tmpDir, "glab-stack", bundled.FileName),
		"default install must not include glab-stack")
}

func TestNewCmdInstall_GlobalFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--global")

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.FileExists(t, filepath.Join(home, skillsRelDir, "glab", bundled.FileName))
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
	assert.FileExists(t, filepath.Join(repoDir, skillsRelDir, "glab", bundled.FileName))
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

	_, err := exec("--path " + tmpDir)
	require.NoError(t, err)

	out, err := exec("--force --path " + tmpDir)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Overwrote")
}

func TestNewCmdInstall_SkipsWithoutForce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)

	_, err := exec("--path " + tmpDir)
	require.NoError(t, err)

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

func TestNewCmdInstall_NamedSkill(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("glab --path " + tmpDir)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")

	dest := filepath.Join(tmpDir, "glab", bundled.FileName)
	assert.FileExists(t, dest)

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: glab")
}

func TestNewCmdInstall_UnknownSkill(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	_, err := exec("does-not-exist --path " + tmpDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown skill "does-not-exist"`)
	assert.Contains(t, err.Error(), "glab skills list")
}

func TestNewCmdInstall_TooManyArgs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	_, err := exec("glab extra --path " + tmpDir)

	require.Error(t, err)
}

func TestComplete(t *testing.T) {
	t.Parallel()

	t.Run("with --path", func(t *testing.T) {
		t.Parallel()

		o := &options{path: "/custom/path"}
		require.NoError(t, o.complete(nil))
		assert.Equal(t, "/custom/path", o.targetDir)
	})

	t.Run("with --global", func(t *testing.T) {
		t.Parallel()

		o := &options{global: true}
		require.NoError(t, o.complete(nil))
		assert.True(t, filepath.IsAbs(o.targetDir), "expected absolute path, got %s", o.targetDir)
		assert.True(t, strings.HasSuffix(o.targetDir, skillsRelDir))
	})
}
