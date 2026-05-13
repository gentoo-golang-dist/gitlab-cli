//go:build !integration

package registry

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	t.Parallel()

	skills, err := All()
	require.NoError(t, err)
	require.NotEmpty(t, skills, "registry should expose at least the bundled skills")

	for i := 1; i < len(skills); i++ {
		assert.Less(t, skills[i-1].Name, skills[i].Name, "registry must be sorted by name")
	}
}

func TestGet_Known(t *testing.T) {
	t.Parallel()

	s, err := Get("glab")
	require.NoError(t, err)
	assert.Equal(t, "glab", s.Name)
	assert.NotEmpty(t, s.SkillFile(), "Get should populate files")
}

func TestGet_Unknown(t *testing.T) {
	t.Parallel()

	_, err := Get("does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound), "error should wrap ErrNotFound")
	assert.Contains(t, err.Error(), `unknown skill "does-not-exist"`)
	assert.Contains(t, err.Error(), "glab skills list")
}
