//go:build !integration

package compile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdConfigCompile(t *testing.T) {
	t.Parallel()

	cmd := NewCmdConfigCompile(cmdtest.NewTestFactory(nil))
	assert.True(t, cmd.Hidden)
	assert.Empty(t, cmd.Example)
	assert.Equal(t, `use "ci lint --render-yaml" instead`, cmd.Deprecated)
}

func TestCompileBehavesLikeLintRenderYAML(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{ID: 123}, nil, nil)
	testClient.MockValidate.EXPECT().
		ProjectNamespaceLint(int64(123), gomock.Any()).
		DoAndReturn(func(pid any, opt *gitlab.ProjectNamespaceLintOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectLintResult, *gitlab.Response, error) {
			require.True(t, *opt.DryRun)
			require.Equal(t, "scratch", *opt.Ref)
			return &gitlab.ProjectLintResult{Valid: true, MergedYaml: "merged:\n  yaml: true\n"}, nil, nil
		})

	exec := cmdtest.SetupCmdForTest(t, NewCmdConfigCompile, false, cmdtest.WithGitLabClient(testClient.Client))
	out, err := exec("--ref scratch ../../lint/testdata/.gitlab-ci.yaml")
	require.NoError(t, err)
	assert.Equal(t, "merged:\n  yaml: true\n", out.OutBuf.String())
	assert.Equal(t, "validating...\n", out.ErrBuf.String())
}
