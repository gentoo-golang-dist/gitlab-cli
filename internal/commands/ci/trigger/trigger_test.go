//go:build !integration

package trigger

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCiTrigger(t *testing.T) {
	t.Parallel()

	createdAt, _ := time.Parse(time.RFC3339, "2022-12-01T05:13:13.703Z")

	// Response indicating last page
	lastPageResponse := &gitlab.Response{
		Response: &http.Response{StatusCode: http.StatusOK},
		NextPage: 0,
	}

	type testCase struct {
		name           string
		args           string
		expectedError  string
		expectedStderr string
		expectedOut    string
		setupMock      func(tc *gitlabtesting.TestClient)
	}

	tests := []testCase{
		{
			name:        "when trigger with job-id",
			args:        "1122",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					Return(&gitlab.Job{
						ID:           1123,
						Status:       "pending",
						Stage:        "build",
						Name:         "build-job",
						Ref:          "branch-name",
						Tag:          false,
						AllowFailure: false,
						CreatedAt:    &createdAt,
						WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
					}, nil, nil)
			},
		},
		{
			name:          "when trigger with job-id throws error",
			args:          "1122",
			expectedError: "403 Forbidden",
			expectedOut:   "",
			setupMock: func(tc *gitlabtesting.TestClient) {
				forbiddenResponse := &gitlab.Response{Response: &http.Response{StatusCode: http.StatusForbidden}}
				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					Return(nil, forbiddenResponse, fmt.Errorf("403 Forbidden"))
			},
		},
		{
			name:        "when trigger with job-name",
			args:        "lint -b main -p 123",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{
						{
							ID:     1122,
							Name:   "lint",
							Status: "failed",
						},
						{
							ID:     1124,
							Name:   "publish",
							Status: "failed",
						},
					}, lastPageResponse, nil)

				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					Return(&gitlab.Job{
						ID:           1123,
						Status:       "pending",
						Stage:        "build",
						Name:         "build-job",
						Ref:          "branch-name",
						Tag:          false,
						AllowFailure: false,
						CreatedAt:    &createdAt,
						WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
					}, nil, nil)
			},
		},
		{
			name:           "when trigger with job-name throws error",
			args:           "lint -b main -p 123",
			expectedError:  "list pipeline jobs: 403 Forbidden",
			expectedStderr: "invalid job ID: lint\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				forbiddenResponse := &gitlab.Response{Response: &http.Response{StatusCode: http.StatusForbidden}}
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
					Return(nil, forbiddenResponse, fmt.Errorf("403 Forbidden"))
			},
		},
		{
			name:        "when trigger with job-name and last pipeline",
			args:        "lint -b main",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				// GetPipelineWithFallback tries GetLatestPipeline first
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", gomock.Any(), gomock.Any()).
					Return(&gitlab.Pipeline{
						ID: 123,
					}, nil, nil)
				// Check if pipeline has jobs
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any()).
					Return([]*gitlab.Job{
						{ID: 1, Name: "test"},
					}, nil, nil)

				// GetJobId lists all jobs for the pipeline
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{
						{
							ID:     1122,
							Name:   "lint",
							Status: "failed",
						},
						{
							ID:     1124,
							Name:   "publish",
							Status: "failed",
						},
					}, lastPageResponse, nil)

				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					Return(&gitlab.Job{
						ID:           1123,
						Status:       "pending",
						Stage:        "build",
						Name:         "build-job",
						Ref:          "branch-name",
						Tag:          false,
						AllowFailure: false,
						CreatedAt:    &createdAt,
						WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
					}, nil, nil)
			},
		},
		{
			name:        "when trigger uses current git branch if no branch specified",
			args:        "lint",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: feature-branch, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				// GetPipelineWithFallback tries GetLatestPipeline for current branch
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", gomock.Any(), gomock.Any()).
					Return(&gitlab.Pipeline{
						ID: 456,
					}, nil, nil)
				// Check if pipeline has jobs
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(456), gomock.Any()).
					Return([]*gitlab.Job{
						{ID: 1, Name: "test"},
					}, nil, nil)

				// GetJobId lists all jobs for the pipeline
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(456), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{
						{
							ID:     1122,
							Name:   "lint",
							Status: "manual",
						},
					}, lastPageResponse, nil)

				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					Return(&gitlab.Job{
						ID:           1123,
						Status:       "pending",
						Stage:        "build",
						Name:         "lint",
						Ref:          "feature-branch",
						Tag:          false,
						AllowFailure: false,
						CreatedAt:    &createdAt,
						WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
					}, nil, nil)
			},
		},
		{
			name:        "when trigger with single variable",
			args:        "1122 --variables DEBUG:true",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					DoAndReturn(func(pid any, jobID int64, opt *gitlab.PlayJobOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Job, *gitlab.Response, error) {
						// Verify variables are passed
						require.NotNil(t, opt.JobVariablesAttributes)
						require.Len(t, *opt.JobVariablesAttributes, 1)
						assert.Equal(t, "DEBUG", *(*opt.JobVariablesAttributes)[0].Key)
						assert.Equal(t, "true", *(*opt.JobVariablesAttributes)[0].Value)
						return &gitlab.Job{
							ID:           1123,
							Status:       "pending",
							Stage:        "build",
							Name:         "build-job",
							Ref:          "branch-name",
							Tag:          false,
							AllowFailure: false,
							CreatedAt:    &createdAt,
							WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
						}, nil, nil
					})
			},
		},
		{
			name:        "when trigger with multiple variables",
			args:        "1122 --variables FOO:bar --variables BAZ:qux",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					DoAndReturn(func(pid any, jobID int64, opt *gitlab.PlayJobOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Job, *gitlab.Response, error) {
						// Verify multiple variables are passed
						require.NotNil(t, opt.JobVariablesAttributes)
						require.Len(t, *opt.JobVariablesAttributes, 2)
						assert.Equal(t, "FOO", *(*opt.JobVariablesAttributes)[0].Key)
						assert.Equal(t, "bar", *(*opt.JobVariablesAttributes)[0].Value)
						assert.Equal(t, "BAZ", *(*opt.JobVariablesAttributes)[1].Key)
						assert.Equal(t, "qux", *(*opt.JobVariablesAttributes)[1].Value)
						return &gitlab.Job{
							ID:           1123,
							Status:       "pending",
							Stage:        "build",
							Name:         "build-job",
							Ref:          "branch-name",
							Tag:          false,
							AllowFailure: false,
							CreatedAt:    &createdAt,
							WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
						}, nil, nil
					})
			},
		},
		{
			name:        "when trigger with variables-env flag",
			args:        "1122 --variables-env KEY:value",
			expectedOut: "Triggered job (ID: 1123), status: pending, ref: branch-name, weburl: https://gitlab.com/OWNER/REPO/-/jobs/1123\n",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockJobs.EXPECT().
					PlayJob("OWNER/REPO", int64(1122), gomock.Any()).
					DoAndReturn(func(pid any, jobID int64, opt *gitlab.PlayJobOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Job, *gitlab.Response, error) {
						// Verify variables are passed
						require.NotNil(t, opt.JobVariablesAttributes)
						require.Len(t, *opt.JobVariablesAttributes, 1)
						assert.Equal(t, "KEY", *(*opt.JobVariablesAttributes)[0].Key)
						assert.Equal(t, "value", *(*opt.JobVariablesAttributes)[0].Value)
						return &gitlab.Job{
							ID:           1123,
							Status:       "pending",
							Stage:        "build",
							Name:         "build-job",
							Ref:          "branch-name",
							Tag:          false,
							AllowFailure: false,
							CreatedAt:    &createdAt,
							WebURL:       "https://gitlab.com/OWNER/REPO/-/jobs/1123",
						}, nil, nil
					})
			},
		},
		{
			name:          "when trigger with invalid variable format",
			args:          "1122 --variables invalid",
			expectedError: "parsing job variable. Expected format KEY:VALUE",
			setupMock:     func(tc *gitlabtesting.TestClient) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN
			testClient := gitlabtesting.NewTestClient(t)
			tc.setupMock(testClient)

			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmdTrigger,
				false,
				cmdtest.WithGitLabClient(testClient.Client),
			)

			// WHEN
			output, err := exec(tc.args)

			// THEN
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			}

			assert.Equal(t, tc.expectedOut, output.String())
			if tc.expectedStderr != "" {
				assert.Equal(t, tc.expectedStderr, output.Stderr())
			} else {
				assert.Empty(t, output.Stderr())
			}
		})
	}
}
