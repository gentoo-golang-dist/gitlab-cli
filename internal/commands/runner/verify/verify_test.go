//go:build !integration

package verify

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestRunnerVerify_Success(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	token := "glrt-abc123"
	tc.MockRunners.EXPECT().
		VerifyRegisteredRunner(
			gomock.AssignableToTypeOf(&gitlab.VerifyRegisteredRunnerOptions{}),
			gomock.Any(),
		).
		DoAndReturn(func(opt *gitlab.VerifyRegisteredRunnerOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
			require.NotNil(t, opt.Token)
			assert.Equal(t, token, *opt.Token)
			return nil, nil
		})

	out, err := exec("--token " + token)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Runner authentication verified successfully")
}

func TestRunnerVerify_MissingToken(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token")
	assert.Contains(t, err.Error(), "required")
}

func TestRunnerVerify_InvalidCredentials(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(tc.Client))),
	)

	tc.MockRunners.EXPECT().
		VerifyRegisteredRunner(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("403 Credentials are invalid"))

	_, err := exec("--token invalid-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}
