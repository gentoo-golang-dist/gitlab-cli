//go:build !integration

package delete

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func noMorePages() *gitlab.Response {
	return &gitlab.Response{NextPage: 0}
}

func TestDeleteServiceAccount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cli         string
		expectedOut string
		wantErr     bool
		wantStderr  string
		setupMock   func(tc *gitlabtesting.TestClient)
	}{
		{
			name:        "delete by numeric ID",
			cli:         "--service-account 100 --group my-group",
			expectedOut: "Deleted service account 100 from group my-group\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					DeleteServiceAccount("my-group", int64(100), gomock.Any()).
					Return(nil, nil)
			},
		},
		{
			name:        "delete by name",
			cli:         "--service-account ci-bot --group my-group",
			expectedOut: "Deleted service account 100 from group my-group\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccounts("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.GroupServiceAccount{
						{ID: 100, Name: "CI Bot", UserName: "ci-bot"},
					}, noMorePages(), nil)
				tc.MockGroups.EXPECT().
					DeleteServiceAccount("my-group", int64(100), gomock.Any()).
					Return(nil, nil)
			},
		},
		{
			name:       "missing group flag",
			cli:        "--service-account 100",
			wantErr:    true,
			wantStderr: "--group",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "missing service-account flag",
			cli:        "--group my-group",
			wantErr:    true,
			wantStderr: "--service-account",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testClient := gitlabtesting.NewTestClient(t)
			tc.setupMock(testClient)
			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmdDelete,
				true,
				cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
			)

			out, err := exec(tc.cli)

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
