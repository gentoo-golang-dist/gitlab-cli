package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

// Mock helpers
func mockBaseRepo(fullName string, err error) func() (glrepo.Interface, error) {
	return func() (glrepo.Interface, error) {
		if err != nil {
			return nil, err
		}
		// Parse "owner/repo" format
		owner, repo, found := strings.Cut(fullName, "/")
		if !found {
			return nil, errors.New("invalid repo format")
		}
		return glrepo.New(owner, repo, "gitlab.com"), nil
	}
}

func TestDetectScope_WithGroupFlag(t *testing.T) {
	t.Parallel()
	// Test Priority 1
	scope, err := DetectScope("my-group", mockBaseRepo("owner/repo", nil))

	require.NoError(t, err)
	assert.Equal(t, api.ScopeTypeGroup, scope.Type)
	assert.Equal(t, "my-group", scope.Path)
}

func TestDetectScope_WithGroupFlag_NestedPath(t *testing.T) {
	t.Parallel()
	// Test nested group paths
	scope, err := DetectScope("parent/child/grandchild", mockBaseRepo("", errors.New("no repo")))

	require.NoError(t, err)
	assert.Equal(t, "group", scope.Type)
	assert.Equal(t, "parent/child/grandchild", scope.Path)
}

func TestDetectScope_FromRepository(t *testing.T) {
	t.Parallel()
	// Test Priority 2
	scope, err := DetectScope("", mockBaseRepo("gitlab-org/cli", nil))

	require.NoError(t, err)
	assert.Equal(t, "project", scope.Type)
	assert.Equal(t, "gitlab-org/cli", scope.Path)
}

func TestDetectScope_NoContext(t *testing.T) {
	t.Parallel()
	// Test Priority 3
	scope, err := DetectScope("", mockBaseRepo("", errors.New("not in repo")))

	require.Error(t, err)
	assert.Nil(t, scope)

	// Verify it's a FlagError
	var flagErr cmdutils.FlagError
	assert.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "unable to determine scope")
	assert.Contains(t, err.Error(), "specify --group")
}

func TestDetectScope_GroupFlagOverrideRepo(t *testing.T) {
	t.Parallel()
	// Test that --group flag takes precedence over repo context
	scope, err := DetectScope("my-group", mockBaseRepo("owner/repo", nil))

	require.NoError(t, err)
	assert.Equal(t, "group", scope.Type)
	assert.Equal(t, "my-group", scope.Path)
}
