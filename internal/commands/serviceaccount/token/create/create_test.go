//go:build !integration

package create

import (
	"encoding/json"
	"testing"
	"time"

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

func parseTime(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

func TestCreateServiceAccountToken(t *testing.T) {
	t.Parallel()
	testToken := &gitlab.PersonalAccessToken{
		ID:        500,
		Name:      "my-token",
		Scopes:    []string{"api"},
		CreatedAt: parseTime("2025-01-01T00:00:00Z"),
		ExpiresAt: new(gitlab.ISOTime(*parseTime("2025-01-31T00:00:00Z"))),
		Active:    true,
		Token:     "glpat-test-token-value",
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
			name:        "create token as text",
			cli:         "my-token --service-account 100 --group my-group --scope api",
			expectedOut: "glpat-test-token-value\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					CreateServiceAccountPersonalAccessToken("my-group", int64(100), gomock.Any()).
					Return(testToken, nil, nil)
			},
		},
		{
			name:     "create token as json",
			cli:      "my-token --service-account 100 --group my-group --scope api --output json",
			wantJSON: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					CreateServiceAccountPersonalAccessToken("my-group", int64(100), gomock.Any()).
					Return(testToken, nil, nil)
			},
		},
		{
			name:        "resolve service account by name",
			cli:         "my-token --service-account ci-bot --group my-group --scope api",
			expectedOut: "glpat-test-token-value\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccounts("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.GroupServiceAccount{
						{ID: 100, Name: "CI Bot", UserName: "ci-bot"},
					}, noMorePages(), nil)
				tc.MockGroups.EXPECT().
					CreateServiceAccountPersonalAccessToken("my-group", int64(100), gomock.Any()).
					Return(testToken, nil, nil)
			},
		},
		{
			name:       "missing scope flag",
			cli:        "my-token --service-account 100 --group my-group",
			wantErr:    true,
			wantStderr: "--scope",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "missing group flag",
			cli:        "my-token --service-account 100 --scope api",
			wantErr:    true,
			wantStderr: "--group",
			setupMock:  func(tc *gitlabtesting.TestClient) {},
		},
		{
			name:       "missing service-account flag",
			cli:        "my-token --group my-group --scope api",
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
				NewCmdCreate,
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
				assert.Equal(t, "my-token", result["name"])
			} else if tc.expectedOut != "" {
				assert.Equal(t, tc.expectedOut, out.OutBuf.String())
			}
		})
	}
}
