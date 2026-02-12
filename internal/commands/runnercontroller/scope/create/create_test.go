//go:build !integration

package create

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tc.MockRunnerControllerScopes.EXPECT().
		AddRunnerControllerInstanceScope(int64(42), gomock.Any()).
		Return(&gitlab.RunnerControllerInstanceLevelScoping{
			CreatedAt: &fixedTime,
			UpdatedAt: &fixedTime,
		}, nil, nil)

	out, err := exec("42 --instance")
	require.NoError(t, err)
	assert.Equal(t, "Added instance-level scope to runner controller 42\n", out.OutBuf.String())
}

func TestCreateJSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tc.MockRunnerControllerScopes.EXPECT().
		AddRunnerControllerInstanceScope(int64(42), gomock.Any()).
		Return(&gitlab.RunnerControllerInstanceLevelScoping{
			CreatedAt: &fixedTime,
			UpdatedAt: &fixedTime,
		}, nil, nil)

	out, err := exec("42 --instance --output json")
	require.NoError(t, err)

	var result gitlab.RunnerControllerInstanceLevelScoping
	err = json.Unmarshal(out.OutBuf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, fixedTime, *result.CreatedAt)
	assert.Equal(t, fixedTime, *result.UpdatedAt)
}

func TestCreateError(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockRunnerControllerScopes.EXPECT().
		AddRunnerControllerInstanceScope(int64(42), gomock.Any()).
		Return(nil, nil, errors.New("API error"))

	_, err := exec("42 --instance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestCreateRequiresInstanceFlag(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "instance" not set`)
}

func TestCreateInvalidID(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("invalid --instance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
