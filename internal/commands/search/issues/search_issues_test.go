//go:build !integration

package issues

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// testIssues returns a slice of fake issues for use across tests.
func testIssues() []*gitlab.Issue {
	createdAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	return []*gitlab.Issue{
		{
			ID:        100,
			IID:       10, // int64 in the struct
			ProjectID: 1,
			State:     "opened",
			Title:     "First issue",
			Labels:    gitlab.Labels{"bug", "backend"},
			WebURL:    "https://gitlab.com/owner/repo/-/issues/10",
			CreatedAt: &createdAt,
		},
		{
			ID:        101,
			IID:       11,
			ProjectID: 1,
			State:     "closed",
			Title:     "Second issue",
			Labels:    gitlab.Labels{"feature"},
			WebURL:    "https://gitlab.com/owner/repo/-/issues/11",
			CreatedAt: &createdAt,
		},
	}
}

// newCmd is a helper that wraps NewCmd to match cmdtest.CmdFunc signature.
func newCmd(f cmdutils.Factory) *cobra.Command {
	return NewCmd(f)
}

// ---- command registration tests -----

func TestNewCmdSearchIssues_Flags(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	f := cmdtest.NewTestFactory(ios)

	cmd := NewCmd(f)
	require.NotNil(t, cmd)

	assert.Equal(t, []string{"issue"}, cmd.Aliases)
	assert.NotNil(t, cmd.Flags().Lookup("group"))
	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("page"))
	assert.NotNil(t, cmd.Flags().Lookup("per-page"))
	assert.NotNil(t, cmd.Flags().Lookup("state"))
	assert.NotNil(t, cmd.Flags().Lookup("confidential"))
	assert.NotNil(t, cmd.Flags().Lookup("search-type"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("repo"))

	// MCP safe annotation must be present
	assert.Equal(t, "true", cmd.Annotations["mcp:safe"])
}

func TestNewCmdSearchIssues_RequiresArg(t *testing.T) {
	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	f := cmdtest.NewTestFactory(ios)

	cmd := NewCmd(f)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
}

// ---- project-level search -----

func TestSearchIssues_ProjectScope_TableOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByProject("OWNER/REPO", "auth bug", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	// Note: shlex splits on spaces, so quote the multi-word query
	out, err := exec(`"auth bug"`)
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "2 issue(s) matching")
	assert.Contains(t, stdout, "#10")
	assert.Contains(t, stdout, "First issue")
	assert.Contains(t, stdout, "(bug, backend)")
	assert.Contains(t, stdout, "#11")
	assert.Contains(t, stdout, "Second issue")
	assert.Contains(t, stdout, "(feature)")
}

func TestSearchIssues_ProjectScope_JSONOutput(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByProject("OWNER/REPO", "auth", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	out, err := exec(`auth --output json`)
	require.NoError(t, err)

	var issues []*gitlab.Issue
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &issues))
	assert.Len(t, issues, 2)
	assert.Equal(t, int64(10), issues[0].IID)
	assert.Equal(t, "First issue", issues[0].Title)
}

func TestSearchIssues_ProjectScope_NoResults(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByProject("OWNER/REPO", "noresults", gomock.Any(), gomock.Any()).
		Return([]*gitlab.Issue{}, nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	out, err := exec(`noresults`)
	require.NoError(t, err)

	assert.Contains(t, out.OutBuf.String(), `No issues found for "noresults"`)
	assert.Contains(t, out.OutBuf.String(), "project OWNER/REPO")
}

// ---- group-level search -----

func TestSearchIssues_GroupScope_TableOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByGroup("my-group", "login", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		// Simulate "no git repo" so the group flag drives scope selection.
		cmdtest.WithBaseRepoError(fmt.Errorf("not a git repo")),
	)

	out, err := exec(`login --group my-group`)
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "2 issue(s) matching")
	assert.Contains(t, stdout, "group my-group")
}

func TestSearchIssues_GroupScope_JSONOutput(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByGroup("my-group", "login", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepoError(fmt.Errorf("not a git repo")),
	)

	out, err := exec(`login --group my-group --output json`)
	require.NoError(t, err)

	var issues []*gitlab.Issue
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &issues))
	assert.Len(t, issues, 2)
}

// ---- instance-level search -----

func TestSearchIssues_InstanceScope_TableOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		Issues("crash", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepoError(fmt.Errorf("not a git repo")),
	)

	out, err := exec(`crash`)
	require.NoError(t, err)

	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "2 issue(s) matching")
	assert.Contains(t, stdout, "the GitLab instance")
	assert.Contains(t, stdout, "#10")
	assert.Contains(t, stdout, "First issue")
}

func TestSearchIssues_InstanceScope_JSONOutput(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		Issues("crash", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepoError(fmt.Errorf("not a git repo")),
	)

	out, err := exec(`crash --output json`)
	require.NoError(t, err)

	var issues []*gitlab.Issue
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &issues))
	assert.Len(t, issues, 2)
}

// ---- flags and filters -----

// TestSearchIssues_ClosedState verifies that passing --state closed forwards
// state=closed as a query parameter to the API. We use the mock's Do() callback
// to fire the RequestOptionFunc against a synthetic URL and inspect the result.
func TestSearchIssues_ClosedState(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	var capturedState string

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByProject("OWNER/REPO", "bug", gomock.Any(), gomock.Any()).
		Do(func(_ any, _ string, _ *gitlab.SearchOptions, opts ...gitlab.RequestOptionFunc) ([]*gitlab.Issue, *gitlab.Response, error) {
			// Apply every RequestOptionFunc to a fake request and read back
			// the resulting query string to confirm state=closed was set.
			req, _ := retryablehttp.NewRequest("GET", "https://gitlab.example.com/api/v4/search", nil)
			for _, opt := range opts {
				_ = opt(req)
			}
			capturedState = req.URL.Query().Get("state")
			return nil, nil, nil
		}).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	out, err := exec(`bug --state closed`)
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "issue(s) matching")
	assert.Equal(t, "closed", capturedState, "expected state=closed to be forwarded as a query param")
}

func TestSearchIssues_Pagination(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockSearch.EXPECT().
		IssuesByProject("OWNER/REPO", "bug", gomock.Any(), gomock.Any()).
		Return(testIssues(), nil, nil)

	exec := cmdtest.SetupCmdForTest(t, newCmd, false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
	)

	out, err := exec(`bug --page 2 --per-page 10`)
	require.NoError(t, err)
	assert.NotEmpty(t, out.OutBuf.String())
}
