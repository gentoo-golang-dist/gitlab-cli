//go:build !integration

package status

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_getPipelineWithFallback(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*gitlabtesting.TestClient)
		branch         string
		wantPipeline   *gitlab.Pipeline
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:   "successfully gets latest pipeline",
			branch: "main",
			setupMocks: func(tc *gitlabtesting.TestClient) {
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("main")}, gomock.Any()).
					Return(&gitlab.Pipeline{ID: 1, Status: "success"}, nil, nil)

				// Mock job check to verify pipeline has jobs
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{{ID: 1, Name: "test"}}, nil, nil)
			},
			wantPipeline: &gitlab.Pipeline{ID: 1, Status: "success"},
			wantErr:      false,
		},
		{
			name:   "falls back to MR pipeline when branch pipeline has no jobs",
			branch: "feature",
			setupMocks: func(tc *gitlabtesting.TestClient) {
				// Latest pipeline found but has no jobs (e.g., external pipeline)
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("feature")}, gomock.Any()).
					Return(&gitlab.Pipeline{ID: 1, Status: "success"}, nil, nil)

				// Mock job check returns empty list
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{}, nil, nil)

				// Find and get MR
				tc.MockMergeRequests.EXPECT().
					ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.BasicMergeRequest{{IID: 1}}, nil, nil)

				tc.MockMergeRequests.EXPECT().
					GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
					Return(&gitlab.MergeRequest{
						BasicMergeRequest: gitlab.BasicMergeRequest{IID: 1},
						HeadPipeline:      &gitlab.Pipeline{ID: 2, Status: "running"},
					}, nil, nil)

				tc.MockPipelines.EXPECT().
					GetPipeline("OWNER/REPO", int64(2), gomock.Any()).
					Return(&gitlab.Pipeline{
						ID:     2,
						Status: "running",
					}, nil, nil)
			},
			wantPipeline: &gitlab.Pipeline{ID: 2, Status: "running"},
			wantErr:      false,
		},
		{
			name:   "falls back to MR pipeline when latest not found",
			branch: "feature",
			setupMocks: func(tc *gitlabtesting.TestClient) {
				// Latest pipeline not found
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("feature")}, gomock.Any()).
					Return(nil, nil, errors.New("not found"))

				// Find and get MR
				tc.MockMergeRequests.EXPECT().
					ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.BasicMergeRequest{{IID: 1}}, nil, nil)

				tc.MockMergeRequests.EXPECT().
					GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
					Return(&gitlab.MergeRequest{
						BasicMergeRequest: gitlab.BasicMergeRequest{IID: 1},
						HeadPipeline:      &gitlab.Pipeline{ID: 2, Status: "running"},
					}, nil, nil)

				tc.MockPipelines.EXPECT().
					GetPipeline("OWNER/REPO", int64(2), gomock.Any()).
					Return(&gitlab.Pipeline{
						ID:     2,
						Status: "running",
					}, nil, nil)
			},
			wantPipeline: &gitlab.Pipeline{ID: 2, Status: "running"},
			wantErr:      false,
		},
		{
			name:   "returns error when no pipeline found",
			branch: "feature",
			setupMocks: func(tc *gitlabtesting.TestClient) {
				// Latest pipeline not found
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("feature")}, gomock.Any()).
					Return(nil, nil, errors.New("not found"))

				// No MRs found
				tc.MockMergeRequests.EXPECT().
					ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.BasicMergeRequest{}, nil, nil)
			},
			wantPipeline:   nil,
			wantErr:        true,
			expectedErrMsg: "no pipeline found for branch feature and failed to find associated merge request",
		},
		{
			name:   "returns error when MR has no pipeline",
			branch: "feature",
			setupMocks: func(tc *gitlabtesting.TestClient) {
				// Latest pipeline not found
				tc.MockPipelines.EXPECT().
					GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("feature")}, gomock.Any()).
					Return(nil, nil, errors.New("not found"))

				// Find MR but no pipeline
				tc.MockMergeRequests.EXPECT().
					ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
					Return([]*gitlab.BasicMergeRequest{{IID: 1}}, nil, nil)
				tc.MockMergeRequests.EXPECT().
					GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
					Return(&gitlab.MergeRequest{
						BasicMergeRequest: gitlab.BasicMergeRequest{IID: 1},
					}, nil, nil)
			},
			wantPipeline:   nil,
			wantErr:        true,
			expectedErrMsg: "no pipeline found. It might not exist yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := gitlabtesting.NewTestClient(t)
			tt.setupMocks(tc)

			// Create test IO streams
			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))

			pipeline, err := ciutils.GetPipelineWithFallback(t.Context(), tc.Client, "OWNER/REPO", tt.branch, ios)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				assert.Nil(t, pipeline)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantPipeline.ID, pipeline.ID)
			assert.Equal(t, tt.wantPipeline.Status, pipeline.Status)
		})
	}
}

func TestCiStatusCommand_NoPrompt(t *testing.T) {
	// Test that the command exits cleanly when NO_PROMPT is enabled
	// and doesn't hang waiting for user input
	tc := gitlabtesting.NewTestClient(t)

	// Mock calls in expected order
	gomock.InOrder(
		// Mock a finished pipeline so the command doesn't loop
		tc.MockPipelines.EXPECT().
			GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("main")}, gomock.Any()).
			Return(&gitlab.Pipeline{ID: 1, Status: "success"}, nil, nil),

		// Mock job check in GetPipelineWithFallback
		tc.MockJobs.EXPECT().
			ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Job{{ID: 1, Name: "test"}}, nil, nil),

		// Mock jobs for the pipeline - need to handle pagination
		tc.MockJobs.EXPECT().
			ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Job{
				{ID: 1, Name: "test", Stage: "test", Status: "success"},
			}, &gitlab.Response{NextPage: 0}, nil),
	)

	exec := cmdtest.SetupCmdForTest(t, NewCmdStatus, true,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBranch("main"),
		// Create custom option to disable prompts
		func(f *cmdtest.Factory) {
			f.IOStub.SetPrompt("true")
		},
	)

	// This should complete without hanging
	_, err := exec("")
	require.NoError(t, err)
}

func TestCiStatusCommand_WithPromptsEnabled_FinishedPipeline(t *testing.T) {
	// Test that the command shows pipeline status and exits cleanly
	// when dealing with a finished pipeline (no interactive prompts needed)
	tc := gitlabtesting.NewTestClient(t)

	// Mock calls in expected order
	gomock.InOrder(
		// Mock a finished pipeline
		tc.MockPipelines.EXPECT().
			GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("main")}, gomock.Any()).
			Return(&gitlab.Pipeline{ID: 1, Status: "success"}, nil, nil),

		// Mock job check in GetPipelineWithFallback
		tc.MockJobs.EXPECT().
			ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Job{{ID: 1, Name: "test"}}, nil, nil),

		// Mock jobs for the pipeline - need to handle pagination
		tc.MockJobs.EXPECT().
			ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Job{
				{ID: 1, Name: "test", Stage: "test", Status: "success"},
			}, &gitlab.Response{NextPage: 0}, nil),
	)

	exec := cmdtest.SetupCmdForTest(t, NewCmdStatus, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBranch("main"),
	)

	// This should complete without hanging since the pipeline is finished
	_, err := exec("")
	require.NoError(t, err)
}

func TestCiStatus_JSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	// Mock a finished pipeline
	tc.MockPipelines.EXPECT().
		GetLatestPipeline("OWNER/REPO", &gitlab.GetLatestPipelineOptions{Ref: new("main")}, gomock.Any()).
		Return(&gitlab.Pipeline{ID: 1, Status: "success", Ref: "main"}, nil, nil)

	// Mock job check in GetPipelineWithFallback
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any()).
		Return([]*gitlab.Job{{ID: 1, Name: "test"}}, nil, nil)

	// Mock jobs for the pipeline with pagination
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
		Return([]*gitlab.Job{
			{ID: 1, Name: "test", Stage: "test", Status: "success"},
		}, &gitlab.Response{NextPage: 0}, nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdStatus, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBranch("main"),
	)

	out, err := exec("--output json")
	require.NoError(t, err)

	assert.Contains(t, out.String(), `"id":1`)
	assert.Contains(t, out.String(), `"status":"success"`)
	assert.Contains(t, out.String(), `"jobs"`)
	assert.Empty(t, out.Stderr())
}

func Test_visualLineCount(t *testing.T) {
	tests := []struct {
		name      string
		frame     string
		termWidth int
		want      int
	}{
		{name: "empty", frame: "", termWidth: 80, want: 0},
		{name: "single line no newline", frame: "abc", termWidth: 80, want: 1},
		{name: "single line trailing newline", frame: "abc\n", termWidth: 80, want: 1},
		{name: "two lines", frame: "a\nb\n", termWidth: 80, want: 2},
		{name: "blank line in middle", frame: "a\n\nb\n", termWidth: 80, want: 3},
		{name: "wraps once", frame: "abcd\n", termWidth: 3, want: 2},
		{name: "wraps twice", frame: "abcdefg\n", termWidth: 3, want: 3},
		{name: "exact width", frame: "abc\n", termWidth: 3, want: 1},
		{name: "ansi escapes excluded", frame: "\x1b[31mabc\x1b[0m\n", termWidth: 3, want: 1},
		{name: "wraps with ansi", frame: "\x1b[31mabcd\x1b[0m\n", termWidth: 3, want: 2},
		{name: "tab expanded to next 8-stop", frame: "a\tb\n", termWidth: 80, want: 1},
		{name: "tabs cause wrap when expanded", frame: "abc\tdef\tghi\n", termWidth: 18, want: 2},
		{name: "no termwidth falls back to 1 per line", frame: "abcdef\nghi\n", termWidth: 0, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visualLineCount([]byte(tt.frame), tt.termWidth)
			assert.Equal(t, tt.want, got)
		})
	}
}
