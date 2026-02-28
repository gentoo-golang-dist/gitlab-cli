//go:build !integration

package list

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDeployTokenList(t *testing.T) {
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
		Scopes:    []string{"read_repository", "read_registry"},
	}

	testCases := []testCase{
		{
			name:        "when no deploy tokens are found shows an empty list",
			cli:         "",
			expectedOut: "\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					ListProjectDeployTokens("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.DeployToken{}, nil, nil)
			},
		},
		{
			name:        "when deploy tokens are found shows a list of tokens",
			cli:         "",
			expectedOut: "ID\tName\tUsername\tScopes\tExpires At\tRevoked\tExpired\n1\tMyToken\tgitlab+deploy-token-1\t[read_repository read_registry]\t2025-06-01 00:00:00 +0000 UTC\tfalse\tfalse\n\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					ListProjectDeployTokens("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.DeployToken{testToken}, nil, nil)
			},
		},
		{
			name:        "when listing group deploy tokens",
			cli:         "--group my-group",
			expectedOut: "ID\tName\tUsername\tScopes\tExpires At\tRevoked\tExpired\n1\tMyToken\tgitlab+deploy-token-1\t[read_repository read_registry]\t2025-06-01 00:00:00 +0000 UTC\tfalse\tfalse\n\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					ListGroupDeployTokens("my-group", gomock.Any()).
					Return([]*gitlab.DeployToken{testToken}, nil, nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// GIVEN
			testClient := gitlabtesting.NewTestClient(t)
			tc.setupMock(testClient)
			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmdList,
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
