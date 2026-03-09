//go:build !integration

package delete

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDeleteProject(t *testing.T) {
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
		DeleteProjectDeployToken("OWNER/REPO", int64(42), gomock.Any()).
		Return(nil, nil)

	out, err := exec("42")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "Deploy token 42 deleted from OWNER/REPO")
}

func TestDeleteGroup(t *testing.T) {
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
		DeleteGroupDeployToken("mygroup", int64(42), gomock.Any()).
		Return(nil, nil)

	out, err := exec("42 --group mygroup")

	require.NoError(t, err)

	assert.Contains(t, out.String(), "Deploy token 42 deleted from group mygroup")
}

func TestDeleteInvalidID(t *testing.T) {
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
