//go:build !integration

package rotate

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

func TestRotateServiceAccountToken(t *testing.T) {
	t.Parallel()
	existingTokens := []*gitlab.PersonalAccessToken{
		{
			ID:        500,
			Name:      "my-token",
			Scopes:    []string{"api"},
			CreatedAt: parseTime("2025-01-01T00:00:00Z"),
			Active:    true,
		},
	}

	rotatedToken := &gitlab.PersonalAccessToken{
		ID:        501,
		Name:      "my-token",
		Scopes:    []string{"api"},
		CreatedAt: parseTime("2025-02-01T00:00:00Z"),
		ExpiresAt: new(gitlab.ISOTime(*parseTime("2025-03-03T00:00:00Z"))),
		Active:    true,
		Token:     "glpat-rotated-token",
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
			name:        "rotate by name as text",
			cli:         "my-token --service-account 100 --group my-group",
			expectedOut: "glpat-rotated-token\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccountPersonalAccessTokens("my-group", int64(100), gomock.Any(), gomock.Any()).
					Return(existingTokens, noMorePages(), nil)
				tc.MockGroups.EXPECT().
					RotateServiceAccountPersonalAccessToken("my-group", int64(100), int64(500), gomock.Any()).
					Return(rotatedToken, nil, nil)
			},
		},
		{
			name:     "rotate as json",
			cli:      "my-token --service-account 100 --group my-group --output json",
			wantJSON: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccountPersonalAccessTokens("my-group", int64(100), gomock.Any(), gomock.Any()).
					Return(existingTokens, noMorePages(), nil)
				tc.MockGroups.EXPECT().
					RotateServiceAccountPersonalAccessToken("my-group", int64(100), int64(500), gomock.Any()).
					Return(rotatedToken, nil, nil)
			},
		},
		{
			name:       "token not found",
			cli:        "nonexistent --service-account 100 --group my-group",
			wantErr:    true,
			wantStderr: "no active token found",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListServiceAccountPersonalAccessTokens("my-group", int64(100), gomock.Any(), gomock.Any()).
					Return([]*gitlab.PersonalAccessToken{}, noMorePages(), nil)
			},
		},
		{
			name:       "missing group flag",
			cli:        "my-token --service-account 100",
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
				NewCmdRotate,
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
