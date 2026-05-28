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

// pointDiscoveryAt swaps the package-level candidate locations in
// installed/installed.go so Discover() walks the supplied scratch
// directory instead of ~/.agents/skills/ and the repo root.
func pointDiscoveryAt(t *testing.T, scratch string) {
	t.Helper()
	// We don't have a direct hook here — the update command calls
	// installed.Discover() which is internal. The cleanest test
	// surface is to populate scratch with a directory matching a
	// real bundled skill name (so the registry recognizes it) and
	// rely on installed.candidateLocationsFn being overridable from
	// the installed package's own tests. For this test we use the
	// fact that bundled.All() exposes real bundled skill names.
	installed.StubCandidateLocations(t, scratch)
}

func TestUpdate_missingNameAndAllErrors(t *testing.T) {
	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("")
	require.Error(t, err)
}

func TestUpdate_nameAndAllConflictErrors(t *testing.T) {
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
