//go:build !integration

package create

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

func TestDeployTokenCreate(t *testing.T) {
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
		Name:      "my-token",
		Username:  "gitlab+deploy-token-1",
		Token:     "abc123xyz",
		ExpiresAt: &expiresAt,
		Revoked:   false,
		Expired:   false,
		Scopes:    []string{"read_repository"},
	}

	testCases := []testCase{
		{
			name:        "create project deploy token",
			cli:         "my-token --scope read_repository",
			expectedOut: "abc123xyz\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					CreateProjectDeployToken("OWNER/REPO", gomock.Any()).
					Return(testToken, nil, nil)
			},
		},
		{
			name:        "create group deploy token",
			cli:         "my-token --scope read_repository --group my-group",
			expectedOut: "abc123xyz\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockDeployTokens.EXPECT().
					CreateGroupDeployToken("my-group", gomock.Any()).
					Return(testToken, nil, nil)
			},
		},
		{
			name:       "missing scope",
			cli:        "my-token",
			wantErr:    true,
			wantStderr: "required flag",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "invalid scope",
			cli:        "my-token --scope invalid_scope",
			wantErr:    true,
			wantStderr: "invalid scope",
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
				NewCmdCreate,
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
