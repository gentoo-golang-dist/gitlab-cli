//go:build !integration

package update

import (
	"encoding/json"
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

func TestUpdateServiceAccount(t *testing.T) {
	t.Parallel()
	updatedSA := &gitlab.GroupServiceAccount{
		ID:       100,
		Name:     "New Name",
		UserName: "ci-bot",
		Email:    "ci-bot@example.com",
	}

	tests := []struct {
		name        string
		cli         string
		expectedOut string
		wantJSON    bool
		wantErr     bool
		wantStderr  string
		setupMock   func(tc *gitlabtesting.TestClient)
	}{
		{
			name:        "update name by numeric ID",
			cli:         "--service-account 100 --group my-group --name \"New Name\"",
			expectedOut: "Updated service account 100 (ci-bot)\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					UpdateServiceAccount("my-group", int64(100), gomock.Any()).
					Return(updatedSA, nil, nil)
			},
		},
		{
			name:     "update as json",
			cli:      "--service-account 100 --group my-group --name \"New Name\" --output json",
			wantJSON: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					UpdateServiceAccount("my-group", int64(100), gomock.Any()).
					Return(updatedSA, nil, nil)
			},
		},
		{
			name:        "update by name resolves via list",
			cli:         "--service-account ci-bot --group my-group --name \"New Name\"",
			expectedOut: "Updated service account 100 (ci-bot)\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccounts("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.GroupServiceAccount{
						{ID: 100, Name: "CI Bot", UserName: "ci-bot"},
					}, noMorePages(), nil)
				tc.MockGroups.EXPECT().
					UpdateServiceAccount("my-group", int64(100), gomock.Any()).
					Return(updatedSA, nil, nil)
			},
		},
		{
			name:       "missing group flag",
			cli:        "--service-account 100 --name test",
			wantErr:    true,
			wantStderr: "--group",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "missing service-account flag",
			cli:        "--group my-group --name test",
			wantErr:    true,
			wantStderr: "--service-account",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "no update fields specified",
			cli:        "--service-account 100 --group my-group",
			wantErr:    true,
			wantStderr: "at least one of",
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
				NewCmdUpdate,
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
			if tc.wantJSON {
				var result map[string]any
				err := json.Unmarshal(out.OutBuf.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, "New Name", result["name"])
			} else if tc.expectedOut != "" {
				assert.Equal(t, tc.expectedOut, out.OutBuf.String())
			}
		})
	}
}
