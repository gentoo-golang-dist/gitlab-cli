//go:build !integration

package list

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestList(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tc.MockRunnerControllerTokens.EXPECT().
		ListRunnerControllerTokens(int64(42), gomock.Any(), gomock.Any()).
		Return([]*gitlab.RunnerControllerToken{
			{
				ID:                 1,
				RunnerControllerID: 42,
				Description:        "Token 1",
				CreatedAt:          &fixedTime,
				UpdatedAt:          &fixedTime,
			},
			{
				ID:                 2,
				RunnerControllerID: 42,
				Description:        "",
				CreatedAt:          &fixedTime,
				UpdatedAt:          &fixedTime,
			},
		}, nil, nil)

	out, err := exec("42")
	require.NoError(t, err)
	output := out.OutBuf.String()
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "Description")
	assert.Contains(t, output, "Token 1")
	assert.Contains(t, output, "-")
}

func TestListJSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tc.MockRunnerControllerTokens.EXPECT().
		ListRunnerControllerTokens(int64(42), gomock.Any(), gomock.Any()).
		Return([]*gitlab.RunnerControllerToken{
			{
				ID:                 1,
				RunnerControllerID: 42,
				Description:        "Token 1",
				CreatedAt:          &fixedTime,
				UpdatedAt:          &fixedTime,
			},
		}, nil, nil)

	out, err := exec("42 --output json")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), `"id":1`)
	assert.Contains(t, out.OutBuf.String(), `"runner_controller_id":42`)
}

func TestListInvalidID(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
