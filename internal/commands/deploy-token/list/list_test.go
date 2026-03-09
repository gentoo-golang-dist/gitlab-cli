//go:build !integration

package list

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestListProject(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	tc.MockDeployTokens.EXPECT().
		ListProjectDeployTokens("OWNER/REPO", &gitlab.ListProjectDeployTokensOptions{
			ListOptions: gitlab.ListOptions{Page: 1, PerPage: api.DefaultListLimit},
		}, gomock.Any()).
		Return([]*gitlab.DeployToken{
			{ID: 1, Name: "token-1", Username: "gitlab+deploy-token-1", Scopes: []string{"read_repository"}},

			{ID: 2, Name: "token-2", Username: "gitlab+deploy-token-2", Scopes: []string{"read_registry"}},
		}, nil, nil)

	out, err := exec("")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "token-1")

	assert.Contains(t, out.String(), "token-2")
}

func TestListGroup(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	tc.MockDeployTokens.EXPECT().
		ListGroupDeployTokens("mygroup", &gitlab.ListGroupDeployTokensOptions{
			ListOptions: gitlab.ListOptions{Page: 1, PerPage: api.DefaultListLimit},
		}, gomock.Any()).
		Return([]*gitlab.DeployToken{
			{ID: 10, Name: "group-token", Username: "gitlab+deploy-token-10", Scopes: []string{"read_repository"}},
		}, nil, nil)

	out, err := exec("--group mygroup")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "group-token")
}

func TestListJSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	tc.MockDeployTokens.EXPECT().
		ListProjectDeployTokens("OWNER/REPO", &gitlab.ListProjectDeployTokensOptions{
			ListOptions: gitlab.ListOptions{Page: 1, PerPage: api.DefaultListLimit},
		}, gomock.Any()).
		Return([]*gitlab.DeployToken{
			{ID: 1, Name: "token-1", Username: "gitlab+deploy-token-1", Scopes: []string{"read_repository"}},
		}, nil, nil)

	out, err := exec("--output json")

	require.NoError(t, err)

	assert.Contains(t, out.String(), `"id":1`)

	assert.Contains(t, out.String(), `"name":"token-1"`)
}

func TestListEmpty(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	tc.MockDeployTokens.EXPECT().
		ListProjectDeployTokens("OWNER/REPO", gomock.Any(), gomock.Any()).
		Return([]*gitlab.DeployToken{}, nil, nil)

	out, err := exec("")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "No deploy tokens found")
}

func TestListPagination(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	tc.MockDeployTokens.EXPECT().
		ListProjectDeployTokens("OWNER/REPO", &gitlab.ListProjectDeployTokensOptions{
			ListOptions: gitlab.ListOptions{Page: 2, PerPage: 5},
		}, gomock.Any()).
		Return([]*gitlab.DeployToken{
			{ID: 3, Name: "token-3", Username: "gitlab+deploy-token-3", Scopes: []string{"read_repository"}},
		}, nil, nil)

	out, err := exec("--page 2 --per-page 5")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "token-3")
}

func TestListInvalidPage(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	_ = tc // unused but needed for test client setup

	_, err := exec("--page 0")

	require.Error(t, err)

	fmt.Println(err)

	assert.Contains(t, err.Error(), "--page must be >= 1")
}
