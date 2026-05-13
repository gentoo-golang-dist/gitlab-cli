//go:build !integration

package bundled

import (
	"errors"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
)

func TestAll(t *testing.T) {
	t.Parallel()

	skills, err := All()
	require.NoError(t, err)
	require.NotEmpty(t, skills)

	for _, s := range skills {
		assert.NotEmpty(t, s.Name, "skill name should be set")
		assert.NotEmpty(t, s.Description, "skill description should be set")
		assert.Equal(t, skill.SourceBundled, s.Source, "Source should be set to bundled")
		assert.NotEmpty(t, s.Files[skill.FileName], "skill must include %s", skill.FileName)
		assert.NotEmpty(t, s.SkillFile(), "SkillFile() must return SKILL.md content")
	}
}

func TestGet_Known(t *testing.T) {
	t.Parallel()

	s, err := Get("glab")
	require.NoError(t, err)
	assert.Equal(t, "glab", s.Name)
	assert.Equal(t, skill.SourceBundled, s.Source)
	assert.NotEmpty(t, s.Description)
	assert.Contains(t, string(s.SkillFile()), "name: glab")
}

func TestGet_Unknown(t *testing.T) {
	t.Parallel()

	_, err := Get("does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound), "error should wrap ErrNotFound")
}

func TestRelPath(t *testing.T) {
	t.Parallel()

	t.Run("relative under root", func(t *testing.T) {
		t.Parallel()
		rel, err := relPath("assets/glab", "assets/glab/SKILL.md")
		require.NoError(t, err)
		assert.Equal(t, "SKILL.md", rel)
	})

	t.Run("nested under root", func(t *testing.T) {
		t.Parallel()
		rel, err := relPath("assets/glab", "assets/glab/scripts/run.sh")
		require.NoError(t, err)
		assert.Equal(t, "scripts/run.sh", rel)
	})

	t.Run("rejects parent traversal", func(t *testing.T) {
		t.Parallel()
		// path.Clean turns "assets/glab/../other" into "assets/other"
		// which doesn't start with "assets/glab/".
		_, err := relPath("assets/glab", path.Clean("assets/glab/../other/SKILL.md"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not under skill root")
	})

	t.Run("rejects sibling directory", func(t *testing.T) {
		t.Parallel()
		_, err := relPath("assets/glab", "assets/glab-stack/SKILL.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not under skill root")
	})

	t.Run("rejects path equal to root", func(t *testing.T) {
		t.Parallel()
		_, err := relPath("assets/glab", "assets/glab")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "equals skill root")
	})

	t.Run("rejects absolute path", func(t *testing.T) {
		t.Parallel()
		_, err := relPath("assets/glab", "/etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not under skill root")
	})
}

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		content := []byte("---\nname: foo\ndescription: bar baz\n---\nbody\n")
		fm, err := parseFrontmatter(content)
		require.NoError(t, err)
		assert.Equal(t, "foo", fm.Name)
		assert.Equal(t, "bar baz", fm.Description)
	})

	t.Run("missing leading delimiter", func(t *testing.T) {
		t.Parallel()

		_, err := parseFrontmatter([]byte("name: foo\n"))
		require.Error(t, err)
	})

	t.Run("missing closing delimiter", func(t *testing.T) {
		t.Parallel()

		_, err := parseFrontmatter([]byte("---\nname: foo\n"))
		require.Error(t, err)
	})
}
