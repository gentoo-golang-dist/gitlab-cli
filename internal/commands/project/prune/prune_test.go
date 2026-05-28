//go:build !integration

package prune

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

const fullName = "OWNER/REPO"

type stubMRs struct {
	bySource map[string][]*gitlab.BasicMergeRequest
}

func (s *stubMRs) handler(_ *gitlab.Client, _ any, opts *gitlab.ListProjectMergeRequestsOptions, _ ...api.CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
	if opts == nil || opts.SourceBranch == nil {
		return nil, nil
	}
	return s.bySource[*opts.SourceBranch], nil
}

func swapListMRs(t *testing.T, fn func(*gitlab.Client, any, *gitlab.ListProjectMergeRequestsOptions, ...api.CliListMROption) ([]*gitlab.BasicMergeRequest, error)) {
	t.Helper()
	orig := api.ListMRs
	api.ListMRs = fn
	t.Cleanup(func() { api.ListMRs = orig })
}

func swapGetProject(t *testing.T, fn func(*gitlab.Client, any) (*gitlab.Project, error)) {
	t.Helper()
	orig := api.GetProject
	api.GetProject = fn
	t.Cleanup(func() { api.GetProject = orig })
}

func expectListLocalBranches(mockGit *git_testing.MockGitRunner, branches ...string) {
	var out strings.Builder
	for _, b := range branches {
		out.WriteString(b + "\n")
	}
	mockGit.EXPECT().
		Git("for-each-ref", "--format=%(refname:short)", "refs/heads/").
		Return(out.String(), nil)
}

func expectNoProtectedBranches(t *testing.T, tc *gitlabtesting.TestClient) {
	t.Helper()
	tc.MockProtectedBranches.EXPECT().
		ListProtectedBranches(fullName, gomock.Any()).
		Return(nil, &gitlab.Response{}, nil)
}

func expectProject(t *testing.T) {
	t.Helper()
	swapGetProject(t, func(_ *gitlab.Client, _ any) (*gitlab.Project, error) {
		return &gitlab.Project{DefaultBranch: "main"}, nil
	})
}

func TestPrune_DeletesBranchesWithMergedMR(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"feature/merged": {{IID: 42, State: "merged", TargetBranch: "main"}},
			"feature/open":   {{IID: 43, State: "opened", TargetBranch: "main"}},
			"feature/empty":  nil,
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "main", "feature/merged", "feature/open", "feature/empty")
	mockGit.EXPECT().Git("branch", "-D", "feature/merged").Return("", nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other-branch"),
	)

	out, err := exec("--yes")
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "feature/merged")
	assert.Contains(t, stdout, "(MR !42 → main)")
	assert.NotContains(t, stdout, "feature/open")
	assert.NotContains(t, stdout, "feature/empty")
	assert.Contains(t, stdout, "deleted feature/merged")
	assert.Contains(t, stdout, "1 branch(es) deleted.")
}

func TestPrune_SkipsBranchWithMergedAndOpenMR(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"feature/reused": {
				{IID: 50, State: "merged", TargetBranch: "main"},
				{IID: 51, State: "opened", TargetBranch: "main"},
			},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "feature/reused")

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other-branch"),
	)

	out, err := exec("--yes")
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "No local branches found")
	assert.NotContains(t, stdout, "feature/reused")
}

func TestPrune_DryRunDoesNotDelete(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"feature/merged": {{IID: 1, State: "merged", TargetBranch: "main"}},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "feature/merged")
	// no Git("branch", "-D", ...) expected

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other"),
	)

	out, err := exec("--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "Dry run: no branches were deleted.")
}

func TestPrune_ExcludesProtectedBranches(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	tc.MockProtectedBranches.EXPECT().
		ListProtectedBranches(fullName, gomock.Any()).
		Return([]*gitlab.ProtectedBranch{
			{Name: "release/*"},
			{Name: "staging"},
		}, &gitlab.Response{}, nil)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"release/1.0": {{IID: 10, State: "merged", TargetBranch: "main"}},
			"staging":     {{IID: 11, State: "merged", TargetBranch: "main"}},
			"plain":       {{IID: 12, State: "merged", TargetBranch: "main"}},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "release/1.0", "staging", "plain")
	mockGit.EXPECT().Git("branch", "-D", "plain").Return("", nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other"),
	)

	out, err := exec("--yes")
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.NotContains(t, stdout, "release/1.0")
	assert.NotContains(t, stdout, "staging")
	assert.Contains(t, stdout, "deleted plain")
}

func TestPrune_ExcludesUserPatterns(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"wip-foo":         {{IID: 1, State: "merged", TargetBranch: "main"}},
			"wip-bar":         {{IID: 2, State: "merged", TargetBranch: "main"}},
			"feature/keep-me": {{IID: 3, State: "merged", TargetBranch: "main"}},
			"demo-branch":     {{IID: 4, State: "merged", TargetBranch: "main"}},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "wip-foo", "wip-bar", "feature/keep-me", "demo-branch")
	mockGit.EXPECT().Git("branch", "-D", "feature/keep-me").Return("", nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other"),
	)

	out, err := exec("--exclude wip-*,demo-branch --yes")
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.NotContains(t, stdout, "wip-")
	assert.NotContains(t, stdout, "demo-branch")
	assert.Contains(t, stdout, "deleted feature/keep-me")
}

func TestPrune_SkipsCurrentBranch(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"current-branch": {{IID: 1, State: "merged", TargetBranch: "main"}},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "current-branch")

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("current-branch"),
	)

	out, err := exec("--yes")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "No local branches found")
}

func TestPrune_IncludeCurrentBranch(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	swapListMRs(t, (&stubMRs{
		bySource: map[string][]*gitlab.BasicMergeRequest{
			"current-branch": {{IID: 7, State: "merged", TargetBranch: "main"}},
		},
	}).handler)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "current-branch")
	mockGit.EXPECT().Git("branch", "-D", "current-branch").Return("", nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("current-branch"),
	)

	out, err := exec("--include-current --yes")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "deleted current-branch")
}

func TestPrune_NonInteractiveRequiresYes(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBranch("main"),
	)

	_, err := exec("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestPrune_ProtectedBranchesAPIFailureIsHardError(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	tc.MockProtectedBranches.EXPECT().
		ListProtectedBranches(fullName, gomock.Any()).
		Return(nil, nil, errors.New("forbidden"))

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBranch("main"),
	)

	_, err := exec("--yes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protected branches")
}

func TestPrune_MergedFlagUsesGit(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	expectProject(t)
	expectNoProtectedBranches(t, tc)

	// api.ListMRs must not be called in --merged mode; assert by returning an error
	// if it is.
	swapListMRs(t, func(*gitlab.Client, any, *gitlab.ListProjectMergeRequestsOptions, ...api.CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
		t.Fatalf("api.ListMRs should not be called in --merged mode")
		return nil, nil
	})

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "ff-merged", "not-merged")
	mockGit.EXPECT().
		Git("branch", "--merged", "main", "--format=%(refname:short)").
		Return("ff-merged\n", nil)
	mockGit.EXPECT().Git("branch", "-D", "ff-merged").Return("", nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other"),
	)

	out, err := exec("--merged --yes")
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "deleted ff-merged")
	assert.NotContains(t, stdout, "deleted not-merged")
}

func TestBranchMatches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern, branch string
		want            bool
	}{
		{"main", "main", true},
		{"main", "develop", false},
		{"wip-*", "wip-foo", true},
		{"wip-*", "feature/wip", false},
		{"release/*", "release/1.0", true},
		{"release/*", "release/", true},
		{"feature/?", "feature/a", true},
		{"feature/?", "feature/abc", false},
	}
	for _, tc := range cases {
		got := branchMatches(tc.pattern, tc.branch)
		assert.Equal(t, tc.want, got, "pattern=%q branch=%q", tc.pattern, tc.branch)
	}
}
