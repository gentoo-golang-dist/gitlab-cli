//go:build !integration

package get

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDeployTokenGet(t *testing.T) {
	type testCase struct {
		name        string
		cli         string
		expectedOut string
		wantErr     bool
		wantStderr  string
		setupMock   func(tc *gitlabtesting.TestClient)
	}

	expiresAt := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	testToken := &gitlab.DeployToken{
		ID:        1,
		Name:      "MyToken",
		Username:  "gitlab+deploy-token-1",
		ExpiresAt: &expiresAt,
		Revoked:   false,
		Expired:   false,
		Scopes:    []string{"read_repository"},
	}

	testCases := []testCase{
		{
			name:        "get project deploy token by ID",
			cli:         "1",
			expectedOut: "ID\tName\tUsername\tScopes\tExpires At\tRevoked\tExpired\n1\tMyToken\tgitlab+deploy-token-1\t[read_repository]\t2025-06-01 00:00:00 +0000 UTC\tfalse\tfalse\n\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					GetProjectDeployToken("OWNER/REPO", int64(1)).
					Return(testToken, nil, nil)
			},
		},
		{
			name:        "get group deploy token by ID",
			cli:         "1 --group my-group",
			expectedOut: "ID\tName\tUsername\tScopes\tExpires At\tRevoked\tExpired\n1\tMyToken\tgitlab+deploy-token-1\t[read_repository]\t2025-06-01 00:00:00 +0000 UTC\tfalse\tfalse\n\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					GetGroupDeployToken("my-group", int64(1)).
					Return(testToken, nil, nil)
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
				NewCmdGet,
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
			assert.Empty(t, out.ErrBuf.String())
		})
	}
}
