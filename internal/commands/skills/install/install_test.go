//go:build !integration

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
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
	assert.FileExists(t, filepath.Join(tmpDir, "glab", skill.FileName))
	// Default install should only ship the core `glab` skill; other
	// bundled skills are opt-in by name to avoid context-window
	// pollution. Guard against regressing back to "install all".
	assert.NoFileExists(t, filepath.Join(tmpDir, "glab-stack", skill.FileName),
		"default install must not include glab-stack")
}

func TestNewCmdInstall_GlobalFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--global")

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Installed")
	assert.FileExists(t, filepath.Join(home, skillsRelDir, "glab", skill.FileName))
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
	assert.FileExists(t, filepath.Join(repoDir, skillsRelDir, "glab", skill.FileName))
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

	dest := filepath.Join(tmpDir, "glab", skill.FileName)
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

func TestInstallOne_WritesAllFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ios, _, _, _ := cmdtest.TestIOStreams()
	o := &options{io: ios, targetDir: tmpDir}

	s := skill.Skill{
		Name:        "demo",
		Description: "demo skill",
		Files: map[string][]byte{
			skill.FileName:                 []byte("---\nname: demo\ndescription: x\n---\nbody\n"),
			"scripts/extract.sh":           []byte("#!/bin/sh\necho hi\n"),
			"references/REFERENCE.md":      []byte("# Reference\n"),
			"assets/templates/default.tpl": []byte("template body\n"),
		},
	}

	require.NoError(t, o.installOne(s))

	for rel, want := range s.Files {
		got, err := os.ReadFile(filepath.Join(tmpDir, "demo", filepath.FromSlash(rel)))
		require.NoError(t, err, "expected %s to exist", rel)
		assert.Equal(t, want, got, "contents differ for %s", rel)
	}
}

func TestNewCmdInstall_NoNameSkipsRemoteSkills(t *testing.T) {
	t.Parallel()

	// "glab skills install" (no name) must only install bundled skills.
	// Remote skills are opt-in by name; `registry.All()` returns them with
	// Files == nil, and before this guarded path the installer would print
	// a false "Installed" success without writing anything to disk.
	tmpDir := t.TempDir()
	exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
	out, err := exec("--path " + tmpDir)

	require.NoError(t, err)

	// The `orbit` skill is a remote-only skill in the curated registry. It
	// should not appear in install output and its directory must not exist.
	combined := out.String() + out.Stderr()
	assert.NotContains(t, combined, "Installed "+filepath.Join(tmpDir, "orbit"))
	assert.NoDirExists(t, filepath.Join(tmpDir, "orbit"))

	// And the bundled skills still install.
	assert.FileExists(t, filepath.Join(tmpDir, "glab", skill.FileName))
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
