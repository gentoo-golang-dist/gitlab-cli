//go:build !integration

package bundled

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	t.Parallel()

	skills, err := All()
	require.NoError(t, err)
	require.NotEmpty(t, skills)

	for _, s := range skills {
		assert.NotEmpty(t, s.Name, "skill name should be set")
		assert.NotEmpty(t, s.Description, "skill description should be set")
		assert.NotEmpty(t, s.Content, "skill content should be set")
	}
}

func TestGet_Known(t *testing.T) {
	t.Parallel()

	s, err := Get("glab")
	require.NoError(t, err)
	assert.Equal(t, "glab", s.Name)
	assert.NotEmpty(t, s.Description)
	assert.Contains(t, string(s.Content), "name: glab")
}

func TestGet_Unknown(t *testing.T) {
	t.Parallel()

	_, err := Get("does-not-exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown skill "does-not-exist"`)
	assert.Contains(t, err.Error(), "glab skills list")
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
