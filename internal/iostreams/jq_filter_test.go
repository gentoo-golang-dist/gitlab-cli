package iostreams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJQFilter_SetParsesValidExpression(t *testing.T) {
	j := &JQFilter{}
	require.NoError(t, j.Set(".foo"))
	assert.True(t, j.IsActive())
	assert.Equal(t, ".foo", j.String())
}

func TestJQFilter_SetRejectsInvalidExpression(t *testing.T) {
	j := &JQFilter{}
	err := j.Set(".[")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --jq expression")
	assert.False(t, j.IsActive(), "filter should not be active when Set failed")
}

func TestJQFilter_SetEmptyClearsFilter(t *testing.T) {
	j := &JQFilter{}
	require.NoError(t, j.Set(".foo"))
	require.NoError(t, j.Set(""))
	assert.False(t, j.IsActive())
	assert.Equal(t, "", j.String())
}

func TestJQFilter_IsActiveZeroValue(t *testing.T) {
	// A freshly constructed JQFilter (e.g. attached to a default IOStreams)
	// is inactive until Set is called.
	j := &JQFilter{}
	assert.False(t, j.IsActive())
}

func TestJQFilter_TypeAndStringSatisfyPflagValue(t *testing.T) {
	// Sanity check that the value satisfies the pflag.Value contract.
	j := &JQFilter{}
	assert.Equal(t, "string", j.Type())
	assert.Equal(t, "", j.String())
}
