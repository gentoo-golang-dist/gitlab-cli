//go:build !integration

package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/installed"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// installedSkill seeds a single bundled-skill directory with the given
// SKILL.md contents and returns the directory path.
func installedSkill(t *testing.T, parent, name, body string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644))
	return dir
}

func pointDiscoveryAt(t *testing.T, scratch string) {
	t.Helper()
	installed.StubCandidateLocations(t, scratch)
}

func TestUpdate_missingNameAndAllErrors(t *testing.T) {
	t.Parallel()
	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("")
	require.Error(t, err)
}

func TestUpdate_nameAndAllConflictErrors(t *testing.T) {
	t.Parallel()
	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("glab --all")
	require.Error(t, err)
}

func TestUpdate_unknownNameErrors(t *testing.T) {
	scratch := t.TempDir()
	pointDiscoveryAt(t, scratch)

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("not-a-real-skill")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestUpdate_overwritesDivergedFiles(t *testing.T) {
	// Use a known bundled skill name so installed.Discover() classifies
	// the on-disk copy as a bundled skill and registry.Get can resolve it.
	bs, err := bundled.All()
	require.NoError(t, err)
	require.NotEmpty(t, bs, "this test relies on at least one bundled skill being available")
	name := bs[0].Name

	scratch := t.TempDir()
	skillDir := installedSkill(t, scratch, name, "old content that doesn't match the embedded version")
	pointDiscoveryAt(t, scratch)

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err = exec(name)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, string(bs[0].Files["SKILL.md"]), string(got))
}

func TestUpdate_skipsAlreadyUpToDate(t *testing.T) {
	bs, err := bundled.All()
	require.NoError(t, err)
	require.NotEmpty(t, bs)
	name := bs[0].Name

	scratch := t.TempDir()
	skillDir := filepath.Join(scratch, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	for rel, content := range bs[0].Files {
		p := filepath.Join(skillDir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, content, 0o644))
	}
	pointDiscoveryAt(t, scratch)

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	out, err := exec(name)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "already up to date")
}

// Regression: writeSkill replaces the directory atomically so files
// that exist on disk but no longer in the source are dropped. Without
// the fix the on-disk hash would perpetually differ from the source
// and the skill would show as outdated even after a successful update.
func TestUpdate_removesStaleFiles(t *testing.T) {
	bs, err := bundled.All()
	require.NoError(t, err)
	require.NotEmpty(t, bs)
	name := bs[0].Name

	scratch := t.TempDir()
	skillDir := installedSkill(t, scratch, name, "stale SKILL.md")
	stalePath := filepath.Join(skillDir, "removed-upstream.txt")
	require.NoError(t, os.WriteFile(stalePath, []byte("ghost"), 0o644))
	pointDiscoveryAt(t, scratch)

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err = exec(name)
	require.NoError(t, err)

	_, err = os.Stat(stalePath)
	assert.True(t, os.IsNotExist(err), "file removed upstream should not persist on disk after update")
}

func TestUpdate_allUpdatesEveryInstalledSkill(t *testing.T) {
	bs, err := bundled.All()
	require.NoError(t, err)
	if len(bs) < 2 {
		t.Skipf("need at least 2 bundled skills to exercise --all, have %d", len(bs))
	}

	scratch := t.TempDir()
	for _, b := range bs {
		installedSkill(t, scratch, b.Name, "stale")
	}
	pointDiscoveryAt(t, scratch)

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	out, err := exec("--all")
	require.NoError(t, err)
	for _, b := range bs {
		assert.Contains(t, out.String(), "Updated "+b.Name)
	}
}
