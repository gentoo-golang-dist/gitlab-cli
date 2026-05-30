//go:build !integration

package list

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

func TestListServiceAccountTokens(t *testing.T) {
	t.Parallel()
	testTokens := []*gitlab.PersonalAccessToken{
		{
			ID:        500,
			Name:      "my-token",
			Scopes:    []string{"api"},
			CreatedAt: parseTime("2025-01-01T00:00:00Z"),
			ExpiresAt: new(gitlab.ISOTime(*parseTime("2025-01-31T00:00:00Z"))),
			Active:    true,
		},
		{
			ID:        501,
			Name:      "another-token",
			Scopes:    []string{"read_registry"},
			CreatedAt: parseTime("2025-01-02T00:00:00Z"),
			ExpiresAt: new(gitlab.ISOTime(*parseTime("2025-02-01T00:00:00Z"))),
			Active:    false,
			Revoked:   true,
		},
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
			cli:  "--service-account 100 --group my-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccountPersonalAccessTokens("my-group", int64(100), gomock.Any(), gomock.Any()).
					Return(testTokens, noMorePages(), nil)
			},
		},
		{
			name:     "list as json",
			cli:      "--service-account 100 --group my-group --output json",
			wantJSON: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccountPersonalAccessTokens("my-group", int64(100), gomock.Any(), gomock.Any()).
					Return(testTokens, noMorePages(), nil)
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
				assert.Equal(t, "500", result[0]["id"])
			} else {
				assert.Contains(t, out.OutBuf.String(), "my-token")
				assert.Contains(t, out.OutBuf.String(), "another-token")
			}
		})
	}
}
