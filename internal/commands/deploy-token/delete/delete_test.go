//go:build !integration

package delete

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDeployTokenDelete(t *testing.T) {
	type testCase struct {
		name        string
		cli         string
		expectedOut string
		wantErr     bool
		wantStderr  string
		setupMock   func(tc *gitlabtesting.TestClient)
	}

	testCases := []testCase{
		{
			name:        "delete project deploy token",
			cli:         "123",
			expectedOut: "Deploy token deleted.\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					DeleteProjectDeployToken("OWNER/REPO", int64(123)).
					Return(nil, nil)
			},
		},
		{
			name:        "delete group deploy token",
			cli:         "123 --group my-group",
			expectedOut: "Deploy token deleted.\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					DeleteGroupDeployToken("my-group", int64(123)).
					Return(nil, nil)
			},
		},
		{
			name:       "invalid token ID",
			cli:        "invalid",
			wantErr:    true,
			wantStderr: "deploy token ID must be an integer",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// GIVEN
			testClient := gitlabtesting.NewTestClient(t)
			tc.setupMock(testClient)
			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmdDelete,
				false,
				cmdtest.WithGitLabClient(testClient.Client),
			)

			// WHEN
			out, err := exec(tc.cli)

			// THEN
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantStderr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out.OutBuf.String())
		})
	}
}
