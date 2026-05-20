//go:build !integration

package listgroup

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCiListGroup(t *testing.T) {
	t.Parallel()

	lastPageResponse := &gitlab.Response{
		NextPage: 0,
	}

	now := time.Now()

	type testCase struct {
		name          string
		args          string
		expectedOut   string
		expectedError string
		setupMock     func(tc *gitlabtesting.TestClient)
	}

	tests := []testCase{
		{
			name:        "lists pipelines for group projects",
			args:        "my-group",
			expectedOut: "Showing pipelines across 2 projects in group my-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.Project{
						{ID: 1, PathWithNamespace: "my-group/project-a"},
						{ID: 2, PathWithNamespace: "my-group/project-b"},
					}, lastPageResponse, nil)

				tc.MockPipelines.EXPECT().
					ListProjectPipelines(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*gitlab.PipelineInfo{
						{ID: 100, Status: "success", Ref: "main", WebURL: "https://gitlab.com/p/100", UpdatedAt: &now},
					}, lastPageResponse, nil).
					Times(2)
			},
		},
		{
			name:        "no projects in group",
			args:        "empty-group",
			expectedOut: "No projects found in group empty-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("empty-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.Project{}, lastPageResponse, nil)
			},
		},
		{
			name:        "no pipelines found",
			args:        "my-group",
			expectedOut: "No pipelines found across 1 projects in group my-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.Project{
						{ID: 1, PathWithNamespace: "my-group/project-a"},
					}, lastPageResponse, nil)

				tc.MockPipelines.EXPECT().
					ListProjectPipelines(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*gitlab.PipelineInfo{}, lastPageResponse, nil)
			},
		},
		{
			name:          "group not found",
			args:          "nonexistent",
			expectedError: "list group projects",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("nonexistent", gomock.Any(), gomock.Any()).
					Return(nil, nil, fmt.Errorf("404 Not Found"))
			},
		},
		{
			name:        "json output",
			args:        "my-group -F json",
			expectedOut: `"project":"my-group/proj"`,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.Project{
						{ID: 1, PathWithNamespace: "my-group/proj"},
					}, lastPageResponse, nil)

				tc.MockPipelines.EXPECT().
					ListProjectPipelines(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*gitlab.PipelineInfo{
						{ID: 42, Status: "success", Ref: "main", WebURL: "https://gitlab.com/p/42"},
					}, lastPageResponse, nil)
			},
		},
		{
			name:        "detail flag fetches full pipeline",
			args:        "my-group --detail",
			expectedOut: "Duration",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGroups.EXPECT().
					ListGroupProjects("my-group", gomock.Any(), gomock.Any()).
					Return([]*gitlab.Project{
						{ID: 1, PathWithNamespace: "my-group/proj"},
					}, lastPageResponse, nil)

				tc.MockPipelines.EXPECT().
					ListProjectPipelines(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*gitlab.PipelineInfo{
						{ID: 42, Status: "success", Ref: "main", WebURL: "https://gitlab.com/p/42", UpdatedAt: &now},
					}, lastPageResponse, nil)

				tc.MockPipelines.EXPECT().
					GetPipeline(gomock.Any(), int64(42), gomock.Any()).
					Return(&gitlab.Pipeline{
						ID:       42,
						Duration: 120,
						User:     &gitlab.BasicUser{Username: "admin"},
					}, nil, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testClient := gitlabtesting.NewTestClient(t)
			tc.setupMock(testClient)

			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmdListGroup,
				false,
				cmdtest.WithGitLabClient(testClient.Client),
			)

			output, err := exec(tc.args)

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			}

			assert.Contains(t, output.String(), tc.expectedOut)
		})
	}
}
