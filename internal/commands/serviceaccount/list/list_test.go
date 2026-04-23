//go:build !integration

package list

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

func TestListServiceAccounts(t *testing.T) {
	t.Parallel()
	testAccounts := []*gitlab.GroupServiceAccount{
		{ID: 100, Name: "CI Bot", UserName: "ci-bot", Email: "ci-bot@example.com"},
		{ID: 101, Name: "Deploy Bot", UserName: "deploy-bot", Email: "deploy-bot@example.com"},
	}

	tests := []struct {
		name       string
		cli        string
		wantJSON   bool
		wantErr    bool
		wantStderr string
		setupMock  func(tc *gitlabtesting.TestClient)
	}{
		{
			name: "list as text",
			cli:  "--group my-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccounts("my-group", gomock.Any(), gomock.Any()).
					Return(testAccounts, noMorePages(), nil)
			},
		},
		{
			name:     "list as json",
			cli:      "--group my-group --output json",
			wantJSON: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccounts("my-group", gomock.Any(), gomock.Any()).
					Return(testAccounts, noMorePages(), nil)
			},
		},
		{
			name:       "missing group flag",
			cli:        "",
			wantErr:    true,
			wantStderr: "--group",
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
				NewCmdList,
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
				var result []map[string]any
				err := json.Unmarshal(out.OutBuf.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, 2)
				assert.Equal(t, "100", result[0]["id"])
			} else {
				assert.Contains(t, out.OutBuf.String(), "ci-bot")
				assert.Contains(t, out.OutBuf.String(), "deploy-bot")
			}
		})
	}
}
