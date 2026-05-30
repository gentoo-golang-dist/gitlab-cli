//go:build !integration

package get

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestGetProject(t *testing.T) {
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
		GetProjectDeployToken("OWNER/REPO", int64(42), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 42,

			Name: "CI Token",

			Username: "gitlab+deploy-token-42",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec("42")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "42")

	assert.Contains(t, out.String(), "CI Token")

	assert.Contains(t, out.String(), "gitlab+deploy-token-42")

	assert.Contains(t, out.String(), "Active")
}

func TestGetGroup(t *testing.T) {
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
		GetGroupDeployToken("mygroup", int64(42), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 42,

			Name: "Group Token",

			Username: "gitlab+deploy-token-42",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec("42 --group mygroup")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "42")

	assert.Contains(t, out.String(), "Group Token")
}

func TestGetJSON(t *testing.T) {
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
		GetProjectDeployToken("OWNER/REPO", int64(42), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 42,

			Name: "CI Token",

			Username: "gitlab+deploy-token-42",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec("42 --output json")

	require.NoError(t, err)

	var result gitlab.DeployToken

	err = json.Unmarshal(out.OutBuf.Bytes(), &result)

	require.NoError(t, err)

	assert.Equal(t, int64(42), result.ID)

	assert.Equal(t, "CI Token", result.Name)
}

func TestGetInvalidID(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	_, err := exec("invalid")

	require.Error(t, err)

	assert.Contains(t, err.Error(), "invalid token ID")
}
