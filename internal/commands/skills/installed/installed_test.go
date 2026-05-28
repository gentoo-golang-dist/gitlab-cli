//go:build !integration

package installed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
)

// stubLocations points the discovery walker at the supplied scratch
// dirs instead of the developer's real ~/ and repo root.
func stubLocations(t *testing.T, locs ...location) {
	t.Helper()
	old := candidateLocationsFn
	candidateLocationsFn = func() []location { return locs }
	t.Cleanup(func() { candidateLocationsFn = old })
}

// writeSkill seeds a single skill directory under dir with the given files.
func writeSkill(t *testing.T, dir, name string, files map[string]string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	for rel, body := range files {
		p := filepath.Join(skillDir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
}

func TestDiscover_findsKnownBundledSkill(t *testing.T) {
	scratch := t.TempDir()
	writeSkill(t, scratch, "glab", map[string]string{
		"SKILL.md":     "stale content",
		"references/x": "extra",
	})
	stubLocations(t, location{dir: scratch, scope: ScopeProject})

	got, err := Discover()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "glab", got[0].Name)
	assert.Equal(t, ScopeProject, got[0].Scope)
	assert.Equal(t, skill.SourceBundled, got[0].Source)
	assert.NotEmpty(t, got[0].Hash)
	assert.Equal(t, "stale content", string(got[0].Files["SKILL.md"]))
}

func TestDiscover_ignoresUnknownDirectories(t *testing.T) {
	scratch := t.TempDir()
	writeSkill(t, scratch, "user-authored", map[string]string{"SKILL.md": "anything"})
	stubLocations(t, location{dir: scratch, scope: ScopeProject})

	got, err := Discover()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDiscover_ignoresDotGlabDirectory(t *testing.T) {
	scratch := t.TempDir()
	writeSkill(t, scratch, ".glab", map[string]string{"bookkeeping.json": "{}"})
	stubLocations(t, location{dir: scratch, scope: ScopeProject})

	got, err := Discover()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDiscover_missingLocationsAreNotErrors(t *testing.T) {
	stubLocations(t, location{dir: filepath.Join(t.TempDir(), "does-not-exist"), scope: ScopeProject})

	got, err := Discover()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDiscover_walksMultipleLocations(t *testing.T) {
	project := t.TempDir()
	global := t.TempDir()
	writeSkill(t, project, "glab", map[string]string{"SKILL.md": "p"})
	writeSkill(t, global, "glab-stack", map[string]string{"SKILL.md": "g"})
	stubLocations(t,
		location{dir: project, scope: ScopeProject},
		location{dir: global, scope: ScopeGlobal},
	)

	got, err := Discover()
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Sorted by name.
	assert.Equal(t, "glab", got[0].Name)
	assert.Equal(t, ScopeProject, got[0].Scope)
	assert.Equal(t, "glab-stack", got[1].Name)
	assert.Equal(t, ScopeGlobal, got[1].Scope)
}

func TestDiscover_sameNameInBothScopes(t *testing.T) {
	project := t.TempDir()
	global := t.TempDir()
	writeSkill(t, project, "glab", map[string]string{"SKILL.md": "project"})
	writeSkill(t, global, "glab", map[string]string{"SKILL.md": "global"})
	stubLocations(t,
		location{dir: project, scope: ScopeProject},
		location{dir: global, scope: ScopeGlobal},
	)

	got, err := Discover()
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Same name, sorted by scope ("global" < "project" lexicographically).
	assert.Equal(t, ScopeGlobal, got[0].Scope)
	assert.Equal(t, ScopeProject, got[1].Scope)
}
