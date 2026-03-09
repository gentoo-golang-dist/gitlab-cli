//go:build !integration

package create

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

func TestCreateProject(t *testing.T) {
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
		CreateProjectDeployToken("OWNER/REPO", gomock.Any(), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 1,

			Name: "CI Token",

			Token: "gldt-xxxxxxxxxxxx",

			Username: "gitlab+deploy-token-1",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec(`"CI Token" --scopes read_repository`)

	require.NoError(t, err)

	assert.Contains(t, out.String(), "Created deploy token for OWNER/REPO")

	assert.Contains(t, out.String(), "gldt-xxxxxxxxxxxx")

	assert.Contains(t, out.String(), "CI Token")
}

func TestCreateGroup(t *testing.T) {
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
		CreateGroupDeployToken("mygroup", gomock.Any(), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 2,

			Name: "Group Token",

			Token: "gldt-yyyyyyyyyyyy",

			Username: "gitlab+deploy-token-2",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec(`"Group Token" --scopes read_repository --group mygroup`)

	require.NoError(t, err)

	assert.Contains(t, out.String(), "Created deploy token for group mygroup")

	assert.Contains(t, out.String(), "gldt-yyyyyyyyyyyy")
}

func TestCreateJSON(t *testing.T) {
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
		CreateProjectDeployToken("OWNER/REPO", gomock.Any(), gomock.Any()).
		Return(&gitlab.DeployToken{
			ID: 1,

			Name: "CI Token",

			Token: "gldt-xxxxxxxxxxxx",

			Username: "gitlab+deploy-token-1",

			Scopes: []string{"read_repository"},
		}, nil, nil)

	out, err := exec(`"CI Token" --scopes read_repository --output json`)

	require.NoError(t, err)

	var result gitlab.DeployToken

	err = json.Unmarshal(out.OutBuf.Bytes(), &result)

	require.NoError(t, err)

	assert.Equal(t, int64(1), result.ID)

	assert.Equal(t, "CI Token", result.Name)

	assert.Equal(t, "gldt-xxxxxxxxxxxx", result.Token)
}

func TestCreateMissingScopes(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	_, err := exec(`"CI Token"`)

	require.Error(t, err)

	assert.Contains(t, err.Error(), "required flag")
}

func TestCreateInvalidExpiresAt(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(

		t,

		NewCmd,

		false,

		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),

		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	_, err := exec(`"CI Token" --scopes read_repository --expires-at invalid`)

	require.Error(t, err)

	assert.Contains(t, err.Error(), "invalid date format")
}
