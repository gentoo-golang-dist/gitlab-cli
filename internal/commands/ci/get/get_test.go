//go:build !integration

package get

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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
	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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
	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success", Duration: 10, FailureReason: "bad timing"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	vs := []*gitlab.PipelineVariable{{Key: "RUN_NIGHTLY_BUILD", VariableType: "env_var", Value: "true"}}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(0, 123).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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

	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// return no variables
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(0, 123).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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
	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	j := []*gitlab.Job{{ID: 1, Name: "build", Status: "success"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// Simulate commit not returning a last pipeline (merged result scenario)
	tc.MockCommits.EXPECT().
		GetCommit("OWNER/REPO", "main", gomock.Any()).
		Return(&gitlab.Commit{LastPipeline: nil}, nil, nil)

	out := []*gitlab.BasicMergeRequest{{IID: 1}}
	tc.MockMergeRequests.EXPECT().
		ListProjectMergeRequests("OWNER/REPO", gomock.Any()).
		Return(out, nil, nil)

	mr := gitlab.MergeRequest{HeadPipeline: &gitlab.Pipeline{ID: 123}}
	tc.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", 1, gomock.Any()).
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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

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
		GetPipeline(10, 456, gomock.Any()).
		Return(&gitlab.Pipeline{ID: 456, ProjectID: 10, Status: "pending"}, nil, nil)

	tc.MockJobs.EXPECT().
		ListPipelineJobs(gomock.Any(), 456, gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*gitlab.Job{}, &gitlab.Response{NextPage: 0}, nil)

	tc.MockPipelines.EXPECT().
		GetPipelineVariables(10, 456, gomock.Any()).
		Return(nil, nil, nil)

	pb, err := fetchDownstreamPipeline(context.Background(), tc.Client, &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 10,
			ID:        456,
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
		GetPipeline(10, 456, gomock.Any()).
		Return(&gitlab.Pipeline{ID: 456, ProjectID: 10}, nil, nil)

	tc.MockJobs.EXPECT().
		ListPipelineJobs(gomock.Any(), 456, gomock.Any(), gomock.Any()).
		Return([]*gitlab.Job{}, &gitlab.Response{NextPage: 0}, nil)

	tc.MockPipelines.EXPECT().
		GetPipelineVariables(10, 456, gomock.Any()).
		Return(nil, nil, errors.New("server error"))

	_, err := fetchDownstreamPipeline(context.Background(), tc.Client, &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 10,
			ID:        456,
		},
	}, true)
	require.Error(t, err)
}

func TestCIGetJSON(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdGet,
		false,
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithGitLabClient(tc.Client),
	)
	// Build pipeline object directly
	p := &gitlab.Pipeline{
		ID:     452959326,
		IID:    452959326,
		Status: "success",
		Source: "push",
		Ref:    "main",
		SHA:    "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:   &gitlab.BasicUser{Username: "test"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 452959326).
		Return(p, nil, nil)

	j := []*gitlab.Job{
		{
			ID:     1,
			Name:   "build",
			Status: "success",
		},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 452959326, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	output, err := exec("-p 452959326 -F json -b main")
	require.NoError(t, err)

	merged := &PipelineMergedResponse{
		Pipeline:  p,
		Jobs:      j,
		Bridges:   nil,
		Variables: nil,
	}
	expectedBytes, err := json.Marshal(merged)

	require.NoError(t, err)

	assert.JSONEq(t, string(expectedBytes), output.String())
	assert.Empty(t, output.Stderr())
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
		ID:     452959326,
		IID:    452959326,
		Status: "success",
		Source: "push",
		Ref:    "main",
		SHA:    "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:   &gitlab.BasicUser{Username: "test"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 452959326).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{
		{ID: 1, Name: "build", Status: "success"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 452959326, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// bridges (one bridge that points to a downstream pipeline)
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 29316529,
			ID:        12345678,
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", 452959326, gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: 0}, nil)

	// child pipeline
	childP := &gitlab.Pipeline{
		ID:        12345678,
		IID:       12345678,
		ProjectID: 29316529,
		Status:    "pending",
		Source:    "push",
		Ref:       "main",
		SHA:       "abcdef0123456789",
		User:      &gitlab.BasicUser{Username: "child"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline(29316529, 12345678, gomock.Any()).
		Return(childP, nil, nil)

	// child jobs
	childJ := []*gitlab.Job{
		{ID: 2, Name: "child-build", Status: "success"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs(29316529, 12345678, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(childJ, &gitlab.Response{NextPage: 0}, nil)

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
				Pipeline:  childP,
				Jobs:      childJ,
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

	// parent pipeline
	p := &gitlab.Pipeline{
		ID:         123,
		IID:        123,
		ProjectID:  5,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{
		{ID: 123, Name: "publish", Status: "failed", FailureReason: "bad timing"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// parent variables
	vs := []*gitlab.PipelineVariable{
		{Key: "RUN_NIGHTLY_BUILD", VariableType: "env_var", Value: "true"},
	}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(5, 123).
		Return(vs, nil, nil)

	// bridges
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 10,
			ID:        456,
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: 0}, nil)

	// child pipeline
	childP := &gitlab.Pipeline{
		ID:         456,
		IID:        456,
		ProjectID:  10,
		Status:     "pending",
		Source:     "push",
		Ref:        "main",
		SHA:        "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:       &gitlab.BasicUser{Username: "test"},
		YamlErrors: "-",
		CreatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		StartedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:  gitlab.Ptr(time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC)),
	}
	tc.MockPipelines.EXPECT().
		GetPipeline(10, 456, gomock.Any()).
		Return(childP, nil, nil)

	// child jobs
	childJ := []*gitlab.Job{
		{ID: 123, Name: "publish", Status: "failed", FailureReason: "bad timing"},
	}
	tc.MockJobs.EXPECT().
		ListPipelineJobs(10, 456, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(childJ, &gitlab.Response{NextPage: 0}, nil)

	// child variables
	childVs := []*gitlab.PipelineVariable{
		{Key: "RUN_DAILY_BUILD", VariableType: "env_var", Value: "true"},
	}
	tc.MockPipelines.EXPECT().
		GetPipelineVariables(10, 456, gomock.Any()).
		Return(childVs, nil, nil)

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
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

# Jobs:
ID	Name	Status	Duration	Failure reason
123	publish	failed	0	bad timing

# Variables:
RUN_NIGHTLY_BUILD:	true

# Child 1 pipeline :
id:	456
status:	pending
source:	push
ref:	main
sha:	0ff3ae198f8601a285adcf5c0fff204ee6fba5fd
tag:	false
yaml Errors:	-
user:	test
created:	2023-10-10 00:00:00 +0000 UTC
started:	2023-10-10 00:00:00 +0000 UTC
updated:	2023-10-10 00:00:00 +0000 UTC

# Child 1 jobs :
ID	Name	Status	Duration	Failure reason
123	publish	failed	0	bad timing

# Child 1 variables :
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
		ID:        123,
		IID:       123,
		ProjectID: 5,
		Status:    "pending",
		Source:    "push",
		Ref:       "main",
		SHA:       "0ff3ae198f8601a285adcf5c0fff204ee6fba5fd",
		User:      &gitlab.BasicUser{Username: "test"},
	}
	tc.MockPipelines.EXPECT().
		GetPipeline("OWNER/REPO", 123).
		Return(p, nil, nil)

	// parent jobs
	j := []*gitlab.Job{{ID: 1, Name: "publish", Status: "pending"}}
	tc.MockJobs.EXPECT().
		ListPipelineJobs("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(j, &gitlab.Response{NextPage: 0}, nil)

	// bridge pointing to downstream pipeline which will fail to fetch
	br := &gitlab.Bridge{
		DownstreamPipeline: &gitlab.PipelineInfo{
			ProjectID: 10,
			ID:        456,
		},
	}
	bs := []*gitlab.Bridge{br}
	tc.MockJobs.EXPECT().
		ListPipelineBridges("OWNER/REPO", 123, gomock.Any(), gomock.Any()).
		Return(bs, &gitlab.Response{NextPage: 0}, nil)

	// downstream pipeline fetch fails
	tc.MockPipelines.EXPECT().
		GetPipeline(10, 456, gomock.Any()).
		Return(nil, nil, errors.New("server error"))

	_, err := exec("-p=123 -b=main --with-downstream-pipelines")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to fetch downstream pipeline for parent_pipeline_id=123 downstream_project_id=10 downstream_pipeline_id=456")
	assert.ErrorContains(t, err, "https://gitlab.com/OWNER/REPO/-/pipelines/456")
}
