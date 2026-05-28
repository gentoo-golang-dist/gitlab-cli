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

	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

const fullName = "OWNER/REPO"

func expectListLocalBranches(mockGit *git_testing.MockGitRunner, branches ...string) {
	var out strings.Builder
	for _, b := range branches {
		out.WriteString(b + "\n")
	}
	mockGit.EXPECT().
		Git("for-each-ref", "--format=%(refname:short)", "refs/heads/").
		Return(out.String(), nil)
}

func expectProject(tc *gitlabtesting.TestClient) {
	tc.MockProjects.EXPECT().
		GetProject(fullName, gomock.Any()).
		Return(&gitlab.Project{DefaultBranch: "main"}, &gitlab.Response{}, nil)
}

func expectNoProtectedBranches(tc *gitlabtesting.TestClient) {
	tc.MockProtectedBranches.EXPECT().
		ListProtectedBranches(fullName, gomock.Any()).
		Return(nil, &gitlab.Response{}, nil)
}

// expectMRsForBranch sets up the per-branch MR lookup. mrs is the slice
// returned on the first (and only) page. The expectation matches by
// SourceBranch only, so the call order across branches doesn't have to
// align with the order expectations are registered.
func expectMRsForBranch(tc *gitlabtesting.TestClient, branch string, mrs []*gitlab.BasicMergeRequest) {
	tc.MockMergeRequests.EXPECT().
		ListProjectMergeRequests(fullName, gomock.Matcher(sourceBranchMatcher(branch))).
		Return(mrs, &gitlab.Response{}, nil)
}

// sourceBranchMatcher matches a *gitlab.ListProjectMergeRequestsOptions
// whose SourceBranch points at the expected branch name. Lets per-branch
// expectations be dispatched by SourceBranch rather than call order.
type sourceBranchMatcher string

func (s sourceBranchMatcher) Matches(x any) bool {
	opts, ok := x.(*gitlab.ListProjectMergeRequestsOptions)
	if !ok || opts == nil || opts.SourceBranch == nil {
		return false
	}
	return *opts.SourceBranch == string(s)
}

func (s sourceBranchMatcher) String() string {
	return "SourceBranch == " + string(s)
}

func TestPrune_DeletesBranchesWithMergedMR(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	expectMRsForBranch(tc, "feature/merged", []*gitlab.BasicMergeRequest{{IID: 42, State: "merged", TargetBranch: "main"}})
	expectMRsForBranch(tc, "feature/open", []*gitlab.BasicMergeRequest{{IID: 43, State: "opened", TargetBranch: "main"}})
	expectMRsForBranch(tc, "feature/empty", nil)

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	expectMRsForBranch(tc, "feature/reused", []*gitlab.BasicMergeRequest{
		{IID: 50, State: "merged", TargetBranch: "main"},
		{IID: 51, State: "opened", TargetBranch: "main"},
	})

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

// TestPrune_OpenMROnLaterPageIsRespected verifies the per-branch MR scan
// follows pagination — an open MR on page 2 must still cause the branch
// to be skipped. Regression test for the unpaginated initial implementation.
func TestPrune_OpenMROnLaterPageIsRespected(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)

	// First page: merged MR + a NextPage hint.
	tc.MockMergeRequests.EXPECT().
		ListProjectMergeRequests(fullName, gomock.Any()).
		Return(
			[]*gitlab.BasicMergeRequest{{IID: 100, State: "merged", TargetBranch: "main"}},
			&gitlab.Response{NextPage: 2},
			nil,
		)
	// Second page: an open MR, which must veto the deletion.
	tc.MockMergeRequests.EXPECT().
		ListProjectMergeRequests(fullName, gomock.Any()).
		Return(
			[]*gitlab.BasicMergeRequest{{IID: 101, State: "opened", TargetBranch: "main"}},
			&gitlab.Response{},
			nil,
		)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	expectListLocalBranches(mockGit, "feature/paginated")

	exec := cmdtest.SetupCmdForTest(t, NewCmdPrune, false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithGitRunner(mockGit),
		cmdtest.WithBranch("other"),
	)

	out, err := exec("--yes")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "No local branches found")
}

func TestPrune_DryRunDoesNotDelete(t *testing.T) {
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	expectMRsForBranch(tc, "feature/merged", []*gitlab.BasicMergeRequest{{IID: 1, State: "merged", TargetBranch: "main"}})

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	tc.MockProtectedBranches.EXPECT().
		ListProtectedBranches(fullName, gomock.Any()).
		Return([]*gitlab.ProtectedBranch{
			{Name: "release/*"},
			{Name: "staging"},
		}, &gitlab.Response{}, nil)
	// Only "plain" reaches the MR lookup; the protected entries are filtered out first.
	expectMRsForBranch(tc, "plain", []*gitlab.BasicMergeRequest{{IID: 12, State: "merged", TargetBranch: "main"}})

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	expectMRsForBranch(tc, "feature/keep-me", []*gitlab.BasicMergeRequest{{IID: 3, State: "merged", TargetBranch: "main"}})

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	// "current-branch" is filtered out before any MR lookup happens.

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	expectMRsForBranch(tc, "current-branch", []*gitlab.BasicMergeRequest{{IID: 7, State: "merged", TargetBranch: "main"}})

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
	t.Parallel()

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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
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
	t.Parallel()

	tc := gitlabtesting.NewTestClient(t)
	expectProject(tc)
	expectNoProtectedBranches(tc)
	// No MR-lookup mock is set up — if --merged accidentally invokes the
	// per-branch API path, gomock will fail the test for an unexpected call.

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
