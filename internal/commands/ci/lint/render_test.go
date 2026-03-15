//go:build !integration

package lint

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestRenderSummary(t *testing.T) {
	t.Parallel()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()

	err := renderLintResult(&options{
		io:   ios,
		path: testdataPath(".gitlab-ci.yaml"),
	}, &gitlab.ProjectLintResult{
		Valid: true,
	})
	require.NoError(t, err)

	assert.Equal(t, "✓ CI/CD YAML is valid!\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestRenderMergedYAML(t *testing.T) {
	t.Parallel()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()

	err := renderLintResult(&options{
		io:           ios,
		path:         testdataPath(".gitlab-ci.yaml"),
		isRenderYAML: true,
	}, &gitlab.ProjectLintResult{
		Valid:      true,
		MergedYaml: "merged-yaml\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "merged-yaml\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestRenderMergedYAMLInvalidButPresent(t *testing.T) {
	t.Parallel()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()

	err := renderLintResult(&options{
		io:           ios,
		path:         testdataPath(".gitlab-ci.yaml"),
		isRenderYAML: true,
	}, &gitlab.ProjectLintResult{
		Valid:      false,
		Errors:     []string{"invalid yaml"},
		MergedYaml: "merged-yaml\n",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, cmdutils.SilentError))

	assert.Equal(t, "merged-yaml\n", stdout.String())
	assert.Equal(t, testdataPath(".gitlab-ci.yaml")+" is invalid.\n1 invalid yaml\n", stderr.String())
}

func TestRenderSummaryInvalid(t *testing.T) {
	t.Parallel()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()

	err := renderLintResult(&options{
		io:   ios,
		path: testdataPath(".gitlab-ci.yaml"),
	}, &gitlab.ProjectLintResult{
		Valid:  false,
		Errors: []string{"invalid yaml"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, cmdutils.SilentError))

	assert.Equal(t, "", stdout.String())
	assert.Equal(t, testdataPath(".gitlab-ci.yaml")+" is invalid.\n1 invalid yaml\n", stderr.String())
}
