//go:build !integration

package delete

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDelete(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockRunnerControllerScopes.EXPECT().
		RemoveRunnerControllerInstanceScope(int64(42), gomock.Any()).
		Return(nil, nil)

	out, err := exec("42 --instance --force")
	require.NoError(t, err)
	assert.Equal(t, "Removed instance-level scope from runner controller 42\n", out.OutBuf.String())
}

func TestDeleteError(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockRunnerControllerScopes.EXPECT().
		RemoveRunnerControllerInstanceScope(int64(42), gomock.Any()).
		Return(nil, errors.New("API error"))

	_, err := exec("42 --instance --force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestDeleteRequiresInstanceFlag(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("42 --force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "instance" not set`)
}

func TestDeleteRequiresForceNonInteractive(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("42 --instance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force required")
}

func TestDeleteInvalidID(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	_, err := exec("invalid --instance --force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
