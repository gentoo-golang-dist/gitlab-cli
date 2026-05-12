//go:build !integration

package list

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdList, false)
	out, err := exec("")

	require.NoError(t, err)
	stdout := out.String()
	assert.Contains(t, stdout, "Name")
	assert.Contains(t, stdout, "Description")
	assert.Contains(t, stdout, "glab")
}

func TestNewCmdList_WrapsLongDescriptionsOnTTY(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdList, true)
	out, err := exec("")

	require.NoError(t, err)
	stdout := out.String()
	// The glab skill description is ~250 chars; under TTY mode the default
	// terminal width is 80 and Wrap=true, so it must fold onto multiple lines.
	assert.GreaterOrEqual(t, strings.Count(stdout, "\n"), 3,
		"expected description to wrap across multiple lines, got:\n%s", stdout)
}

func TestNewCmdList_RejectsArgs(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdList, false)
	_, err := exec("unexpected")

	require.Error(t, err)
}
