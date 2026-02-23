//go:build !integration

package get

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCIGet_ExistingPipeline(t *testing.T) {
	t.Parallel()

	// Create gitlab testing client and set gomock expectations
	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// Prepare factory with mocked gitlab client
	output, err := exec("-p=123 -b=main")

	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_MissingPipeline(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	commit := &gitlab.Commit{LastPipeline: &gitlab.PipelineInfo{ID: 123}}
	tc.MockCommits.EXPECT().
		GetCommit("OWNER/REPO", "main", gomock.Any()).
		Return(commit, nil, nil)

	output, err := exec("-b=main")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_WithJobText(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	output, err := exec("-p=123 -b=main")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_WithJobDetails(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success", Duration: 10, FailureReason: "bad timing"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	output, err := exec("-p=123 -b=main --with-job-details")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
ID	Name	Status	Duration	Failure reason
1	build	success	10	bad timing

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_WithVariables(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         int64(123),
		IID:        int64(123),
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	vs := []*gitlab.PipelineVariable{{Key: "RUN_NIGHTLY_BUILD", VariableType: "env_var", Value: "true"}}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(0), int64(123)).
		Return(vs, nil, nil)

	output, err := exec("-p=123 -b=main --with-variables")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

# Variables:
RUN_NIGHTLY_BUILD:	true

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_WithVariablesNone(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         int64(123),
		IID:        int64(123),
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// return no variables
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(0), int64(123)).
		Return([]*gitlab.PipelineVariable{}, nil, nil)

	output, err := exec("-p=123 -b=main --with-variables")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

# Variables:
No variables found in pipeline.
`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGet_MergedResultNoCommitPipeline(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	p := &gitlab.Pipeline{
		ID:         int64(123),
		IID:        int64(123),
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: int64(0)}, nil)

	// Simulate commit not returning a last pipeline (merged result scenario)
	tc.MockCommits.EXPECT().
		GetCommit("OWNER/REPO", "main", gomock.Any()).
		Return(&gitlab.Commit{LastPipeline: nil}, nil, nil)

	out := []*gitlab.BasicMergeRequest{{IID: int64(1)}}
	tc.MockMergeRequests.EXPECT().
		ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
		Return(out, nil, nil)

	mr := gitlab.MergeRequest{HeadPipeline: &gitlab.Pipeline{ID: int64(123)}}
	tc.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
		Return(&mr, nil, nil)

	output, err := exec("-b=main")
	require.Nil(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
build:	success

`

	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestFetchDownstreamPipeline_NoVariablesRequested(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	tc.MockPipelines.EXPECT().
		GetPipeline(int64(10), int64(456), gomock.Any()).
		Return(&gitlab.Pipeline{ID: int64(456), ProjectID: int64(10), Status: "pending"}, nil, nil)

	tc.MockJobs.EXPECT().
		ListPipelineJobs(gomock.Any(), int64(456), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*gitlab.Job{}, &gitlab.Response{NextPage: int64(0)}, nil)
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(10), int64(456), gomock.Any()).
		Return(nil, nil, nil)

	pb, err := fetchDownstreamPipeline(t.Context(), tc.Client, &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: int64(10),
			ID:        int64(456),
		},
	}, true)

	require.NoError(t, err)
	require.NotNil(t, pb.Pipeline)
	assert.Len(t, pb.Jobs, 0)
	assert.Nil(t, pb.Variables)
}

func TestFetchDownstreamPipeline_VariablesError(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	tc.MockPipelines.EXPECT().
		GetPipeline(int64(10), int64(456), gomock.Any()).
		Return(&gitlab.Pipeline{ID: int64(456), ProjectID: int64(10)}, nil, nil)

	tc.MockJobs.EXPECT().
		ListPipelineJobs(gomock.Any(), int64(456), gomock.Any(), gomock.Any()).
		Return([]*gitlab.Job{}, &gitlab.Response{NextPage: int64(0)}, nil)
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(10), int64(456), gomock.Any()).
		Return(nil, nil, errors.New("server error"))

	_, err := fetchDownstreamPipeline(t.Context(), tc.Client, &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 10,
			ID:        456,
		},
	}, true)
	require.Error(t, err)
}

func TestCIGetJSON(t *testing.T) {
	t.Parallel()

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")
	finishedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.35Z")

	jobCreatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.291Z")
	jobStartedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.693Z")
	jobFinishedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.274Z")

	// Response indicating last page
	lastPageResponse := &gitlab.Response{
		Response: &http.Response{StatusCode: http.StatusOK},
		NextPage: 0,
	}

	type testCase struct {
		name      string
		args      string
		setupMock func(tc *gitlabtesting.TestClient)
	}

	tests := []testCase{
		{
			name: "when getting JSON for pipeline",
			args: "-p 452959326 -F json -b main",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockPipelines.EXPECT().
					GetPipeline("OWNER/REPO", int64(452959326)).
					Return(&gitlab.Pipeline{
						ID:         452959326,
						IID:        14,
						ProjectID:  29316529,
						SHA:        "44eb489568f7cb1a5a730fce6b247cd3797172ca",
						Ref:        "1-fake-issue-3",
						Status:     "success",
						Source:     "push",
						CreatedAt:  &createdAt,
						UpdatedAt:  &updatedAt,
						StartedAt:  &startedAt,
						FinishedAt: &finishedAt,
						BeforeSHA:  "001eb421e586a3f07f90aea102c8b2d4068ab5b6",
						Tag:        false,
						User: &gitlab.BasicUser{
							ID:        8814129,
							Username:  "OWNER",
							Name:      "Some User",
							State:     "active",
							Locked:    false,
							AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/8814129/avatar.png",
							WebURL:    "https://gitlab.com/OWNER",
						},
						WebURL:         "https://gitlab.com/OWNER/REPO/-/pipelines/452959326",
						Duration:       14,
						QueuedDuration: 1,
						DetailedStatus: &gitlab.DetailedStatus{
							Icon:        "status_success",
							Text:        "Passed",
							Label:       "passed",
							Group:       "success",
							Tooltip:     "passed",
							HasDetails:  true,
							DetailsPath: "/OWNER/REPO/-/pipelines/452959326",
							Favicon:     "/assets/ci_favicons/favicon_status_success-8451333011eee8ce9f2ab25dc487fe24a8758c694827a582f17f42b0a90446a2.png",
						},
					}, nil, nil)
				tc.MockJobs.EXPECT().
					ListPipelineJobs("OWNER/REPO", int64(452959326), gomock.Any(), gomock.Any()).
					Return([]*gitlab.Job{
						{
							ID:             1999017704,
							Status:         "success",
							Stage:          "test",
							Name:           "test_vars",
							Ref:            "1-fake-issue-3",
							Tag:            false,
							AllowFailure:   false,
							CreatedAt:      &jobCreatedAt,
							StartedAt:      &jobStartedAt,
							FinishedAt:     &jobFinishedAt,
							Duration:       14.580467,
							QueuedDuration: 0.211715,
							User: &gitlab.User{
								ID:        8814129,
								Username:  "OWNER",
								Name:      "Some User",
								State:     "active",
								Locked:    false,
								AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/8814129/avatar.png",
								WebURL:    "https://gitlab.com/OWNER",
							},
							Commit: &gitlab.Commit{
								ID:             "44eb489568f7cb1a5a730fce6b247cd3797172ca",
								ShortID:        "44eb4895",
								Title:          "Add new file",
								AuthorName:     "Some User",
								AuthorEmail:    "OWNER@gitlab.com",
								CommitterName:  "Some User",
								CommitterEmail: "OWNER@gitlab.com",
								Message:        "Add new file",
								ParentIDs:      []string{"001eb421e586a3f07f90aea102c8b2d4068ab5b6"},
								WebURL:         "https://gitlab.com/OWNER/REPO/-/commit/44eb489568f7cb1a5a730fce6b247cd3797172ca",
							},
							Pipeline: gitlab.JobPipeline{
								ID:        452959326,
								ProjectID: 29316529,
								Sha:       "44eb489568f7cb1a5a730fce6b247cd3797172ca",
								Ref:       "1-fake-issue-3",
								Status:    "success",
							},
							WebURL: "https://gitlab.com/OWNER/REPO/-/jobs/1999017704",
							Runner: gitlab.JobRunner{
								ID:          12270859,
								Description: "5-green.saas-linux-small-amd64.runners-manager.gitlab.com/default",
								Active:      true,
								IsShared:    true,
								Name:        "gitlab-runner",
							},
							Artifacts: []gitlab.JobArtifact{
								{
									FileType: "trace",
									Filename: "job.log",
									Size:     2770,
								},
							},
						},
					}, lastPageResponse, nil)
			},
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
				NewCmdGet,
				false,
				cmdtest.WithGitLabClient(testClient.Client),
			)

			// WHEN
			output, err := exec(tc.args)

			// THEN
			require.NoError(t, err)
			// Verify it's valid JSON that contains expected fields
			assert.Contains(t, output.String(), `"id":452959326`)
			assert.Contains(t, output.String(), `"status":"success"`)
			assert.Contains(t, output.String(), `"jobs":[`)
			assert.Contains(t, output.String(), `"test_vars"`)
			assert.Empty(t, output.Stderr())
		})
	}
}

func TestCIGetJSONWithBridges(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t, gitlab.WithBaseURL("https://gitlab.com/api/v4"))
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	// parent pipeline object
	p := &gitlab.Pipeline{
		ID:     int64(452959326),
		IID:    int64(452959326),
		Status: "success",
		Source: "push",
		Ref:    "main",
		SHA:    "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:   &gitlab.BasicUser{Username: "test"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(452959326)).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{
		{ID: 1, Name: "build", Status: "success"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(452959326), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: int64(0)}, nil)

	// bridges (one bridge that points to a downstream pipeline)
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: int64(29316529),
			ID:        int64(12345678),
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", int64(452959326), gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: int64(0)}, nil)

	// downstream pipeline
	downstreamP := &gitlab.Pipeline{
		ID:        int64(12345678),
		IID:       int64(12345678),
		ProjectID: int64(29316529),
		Status:    "pending",
		Source:    "push",
		Ref:       "main",
		SHA:       "abcdef0123456789",
		User:      &gitlab.BasicUser{Username: "child"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline(int64(29316529), int64(12345678), gomock.Any()).
		Return(downstreamP, nil, nil)

	// downstream jobs
	downstreamJ := []*gitlab.Job{
		{ID: 2, Name: "child-build", Status: "success"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs(int64(29316529), int64(12345678), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(downstreamJ, &gitlab.Response{NextPage: int64(0)}, nil)

	output, err := exec("-p 452959326 -F json -b main --with-downstream-pipelines")
	require.NoError(t, err)

	// Build expected merged response
	expected := &PipelineMergedResponse{
		Pipeline:  p,
		Jobs:      j,
		Variables: nil,
		Bridges: []PipelineBridge{
			{
				Bridge:    br,
				Pipeline:  downstreamP,
				Jobs:      downstreamJ,
				Variables: nil,
			},
		},
	}
	expectedBytes, err := json.Marshal(expected)
	require.NoError(t, err)

	assert.JSONEq(t, string(expectedBytes), output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGetWithBridges(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t, gitlab.WithBaseURL("https://gitlab.com/api/v4"))
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	createdAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:16.276Z")
	startedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:17.448Z")
	updatedAt, _ := time.Parse(time.RFC3339, "2022-01-20T21:47:31.358Z")

	// parent pipeline
	p := &gitlab.Pipeline{
		ID:         int64(123),
		IID:        int64(123),
		ProjectID:  int64(5),
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{
		{ID: 123, Name: "publish", Status: "failed", FailureReason: "bad timing"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: int64(0)}, nil)

	// parent variables
	vs := []*gitlab.PipelineVariable{
		{Key: "RUN_NIGHTLY_BUILD", VariableType: "env_var", Value: "true"},
	}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(5), int64(123)).
		Return(vs, nil, nil)

	// bridges
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: int64(10),
			ID:        int64(456),
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: int64(0)}, nil)

	// downstream pipeline
	downstreamP := &gitlab.Pipeline{
		ID:         int64(456),
		IID:        int64(456),
		ProjectID:  int64(10),
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  &createdAt,
		StartedAt:  &startedAt,
		UpdatedAt:  &updatedAt,
	}
	tc.MockPipelines.EXPECT().
		GetPipeline(int64(10), int64(456), gomock.Any()).
		Return(downstreamP, nil, nil)

	// downstream jobs
	downstreamJ := []*gitlab.Job{
		{ID: 123, Name: "publish", Status: "failed", FailureReason: "bad timing"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs(int64(10), int64(456), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(downstreamJ, &gitlab.Response{NextPage: 0}, nil)

	// downstream variables
	downstreamVs := []*gitlab.PipelineVariable{
		{Key: "RUN_DAILY_BUILD", VariableType: "env_var", Value: "true"},
	}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(int64(10), int64(456), gomock.Any()).
		Return(downstreamVs, nil, nil)

	output, err := exec("-p=123 -b=main --with-downstream-pipelines --with-job-details --with-variables")
	require.NoError(t, err)

	expectedOut := `# Pipeline:
id:	123
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Jobs:
ID	Name	Status	Duration	Failure reason
123	publish	failed	0	bad timing

# Variables:
RUN_NIGHTLY_BUILD:	true

# Downstream 1 pipeline :
id:	456
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2022-01-20 21:47:16.276 +0000 UTC
started:	2022-01-20 21:47:17.448 +0000 UTC
updated:	2022-01-20 21:47:31.358 +0000 UTC

# Downstream 1 jobs :
ID	Name	Status	Duration	Failure reason
123	publish	failed	0	bad timing

# Downstream 1 variables :
RUN_DAILY_BUILD:	true

`
	assert.Equal(t, expectedOut, output.String())
	assert.Empty(t, output.Stderr())
}

func TestCIGetWithBridges_DownstreamErrorIsWrapped(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t, gitlab.WithBaseURL("https://gitlab.com/api/v4"))
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)

	// parent pipeline
	p := &gitlab.Pipeline{
		ID:        int64(123),
		IID:       int64(123),
		ProjectID: int64(5),
		Status:    "pending",
		Source:    "push",
		Ref:       "main",
		SHA:       "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:      &gitlab.BasicUser{Username: "test"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", int64(123)).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{{ID: 1, Name: "publish", Status: "pending"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: int64(0)}, nil)

	// bridge pointing to downstream pipeline which will fail to fetch
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: int64(10),
			ID:        int64(456),
			WebURL:    "https://gitlab.com/OWNER/REPO/-/pipelines/456",
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: int64(0)}, nil)

	// downstream pipeline fetch fails
	tc.MockPipelines.EXPECT().
		GetPipeline(int64(10), int64(456), gomock.Any()).
		Return(nil, nil, errors.New("server error"))

	_, err := exec("-p=123 -b=main --with-downstream-pipelines")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to fetch downstream pipeline for parent_pipeline_id=123 downstream_project_id=10 downstream_pipeline_id=456")
	assert.ErrorContains(t, err, "https://gitlab.com/OWNER/REPO/-/pipelines/456")
}
