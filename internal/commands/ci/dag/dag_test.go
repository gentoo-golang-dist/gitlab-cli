//go:build !integration

package dag

import (
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), "testdata", name)
}

func TestNewCmdDag_InvalidPath(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	_, err := exec("NONEXISTENT_FILE")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestNewCmdDag_EmptyPipeline(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	_, err := exec(testdataPath("empty.yml"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no jobs found")
}

func TestNewCmdDag_SVGOutput(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	result, err := exec(testdataPath("simple.yml") + " --output svg")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
	assert.Contains(t, result.String(), "</svg>")
}

func TestNewCmdDag_SVGOutputWithNeeds(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	result, err := exec(testdataPath("needs.yml") + " --output svg")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
}

func TestNewCmdDag_SVGOutputWithExtends(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	result, err := exec(testdataPath("extends.yml") + " --output svg")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
}

func TestNewCmdDag_InvalidOutputFlag(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	_, err := exec(testdataPath("simple.yml") + " --output json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output mode")
}

func TestNewCmdDag_NoStageEdges(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdDag, false)

	result, err := exec(testdataPath("simple.yml") + " --output svg --stage-edges=false")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
}
