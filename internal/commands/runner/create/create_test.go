//go:build !integration

package create

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestRunnerCreate_Success(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	expires := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tc.MockUsers.EXPECT().
		CreateUserRunner(gomock.Any(), gomock.Any()).
		Return(&gitlab.UserRunner{ID: 12345, Token: "glrt-testtoken", TokenExpiresAt: &expires}, nil, nil)

	out, err := exec(`--runner-type instance_type --description "ci runner"`)
	require.NoError(t, err)

	expectedOutput := heredoc.Doc(`
		Created runner 12345
		Authentication token: glrt-testtoken
		Token expires at: 2026-01-01T00:00:00Z
	`)
	assert.Equal(t, expectedOutput, out.OutBuf.String())
}

func TestRunnerCreate_OutputJSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockUsers.EXPECT().
		CreateUserRunner(gomock.Any(), gomock.Any()).
		Return(&gitlab.UserRunner{ID: 99, Token: "tok"}, nil, nil)

	out, err := exec("--runner-type instance_type --output json")
	require.NoError(t, err)
	var got gitlab.UserRunner
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &got))
	assert.Equal(t, int64(99), got.ID)
	assert.Equal(t, "tok", got.Token)
}

func TestRunnerCreate_MissingRunnerType(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner-type")
}

func TestRunnerCreate_EmptyRunnerType(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec(`--runner-type ""`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestRunnerCreate_APIError(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockUsers.EXPECT().
		CreateUserRunner(gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("403 Forbidden"))

	_, err := exec("--runner-type instance_type")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestRunnerCreate_GroupTypeRequiresGroupID(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("--runner-type group_type")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group-id")
}

func TestRunnerCreate_ProjectTypeRequiresProjectID(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("--runner-type project_type")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project-id")
}

func TestRunnerCreate_InstanceTypeRejectsGroupOrProjectID(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("--runner-type instance_type --group-id 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instance_type")
}
