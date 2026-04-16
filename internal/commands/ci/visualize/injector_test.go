//go:build !integration

package visualize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseForAssert(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(content, &raw))
	return raw
}

func TestInjectVariables_NoVars_Passthrough(t *testing.T) {
	t.Parallel()

	in := []byte("stages:\n  - build\njob:\n  script: echo hi\n")
	out, err := injectVariables(in, nil)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestInjectVariables_AddsBlockWhenMissing(t *testing.T) {
	t.Parallel()

	in := []byte("stages:\n  - test\njob:\n  script: echo hi\n")
	out, err := injectVariables(in, map[string]string{"FOO": "bar"})
	require.NoError(t, err)

	parsed := parseForAssert(t, out)
	vars, ok := parsed["variables"].(map[string]any)
	require.True(t, ok, "expected variables block, got %#v", parsed["variables"])
	assert.Equal(t, "bar", vars["FOO"])
	// Original content preserved.
	assert.NotNil(t, parsed["stages"])
	assert.NotNil(t, parsed["job"])
}

func TestInjectVariables_MergesIntoExistingBlock(t *testing.T) {
	t.Parallel()

	in := []byte("variables:\n  EXISTING: keep\n  OVERRIDE: old\nstages:\n  - test\n")
	out, err := injectVariables(in, map[string]string{
		"OVERRIDE": "new",
		"ADDED":    "value",
	})
	require.NoError(t, err)

	parsed := parseForAssert(t, out)
	vars := parsed["variables"].(map[string]any)
	assert.Equal(t, "keep", vars["EXISTING"], "existing unrelated var preserved")
	assert.Equal(t, "new", vars["OVERRIDE"], "user flag wins over existing")
	assert.Equal(t, "value", vars["ADDED"])
}

func TestInjectVariables_CIReservedVarsAccepted(t *testing.T) {
	t.Parallel()

	in := []byte("stages:\n  - test\njob:\n  script: echo\n")
	out, err := injectVariables(in, map[string]string{
		"CI_PIPELINE_SOURCE":                  "merge_request_event",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
	})
	require.NoError(t, err)

	parsed := parseForAssert(t, out)
	vars := parsed["variables"].(map[string]any)
	assert.Equal(t, "merge_request_event", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_MERGE_REQUEST_TARGET_BRANCH_NAME"])
}

func TestInjectVariables_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := injectVariables([]byte("not: [valid: yaml"), map[string]string{"X": "y"})
	assert.Error(t, err)
}
