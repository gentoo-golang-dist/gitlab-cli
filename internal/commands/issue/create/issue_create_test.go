//go:build !integration

package create

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestGenerateIssueWebURL(t *testing.T) {
	opts := &options{
		Labels:         []string{"backend", "frontend"},
		Assignees:      []string{"johndoe", "janedoe"},
		Milestone:      15,
		Weight:         3,
		IsConfidential: true,
		baseProject: &gitlab.Project{
			ID:     101,
			WebURL: "https://gitlab.example.com/gitlab-org/gitlab",
		},
		Title: "Autofill tests | for this @project",
	}

	u, err := generateIssueWebURL(opts)

	expectedUrl := "https://gitlab.example.com/gitlab-org/gitlab/-/issues/new?" +
		"issue%5Bdescription%5D=%0A%2Flabel+~%22backend%22+~%22frontend%22%0A%2Fassign+johndoe%2C+janedoe%0A%2Fmilestone+%2515%0A%2Fweight+3%0A%2Fconfidential&" +
		"issue%5Btitle%5D=Autofill+tests+%7C+for+this+%40project"

	assert.NoError(t, err)
	assert.Equal(t, expectedUrl, u)
}

func TestIssueCreate_WithTemplate(t *testing.T) {
	// Cannot use t.Parallel(): test mutates git.ToplevelDir, a package-level variable.
	t.Setenv("NO_COLOR", "true")

	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, ".gitlab", "issue_templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "bug.md"), []byte("## Bug Report\n\nDescribe the bug here."), 0o644))

	origToplevelDir := git.ToplevelDir
	git.ToplevelDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { git.ToplevelDir = origToplevelDir })

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                37777023,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
		}, nil, nil)
	testClient.MockIssues.EXPECT().
		CreateIssue("OWNER/REPO", gomock.Any()).
		DoAndReturn(func(_ any, opts *gitlab.CreateIssueOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.Issue, *gitlab.Response, error) {
			assert.Equal(t, "## Bug Report\n\nDescribe the bug here.", *opts.Description)
			return &gitlab.Issue{IID: 1, Title: "Test Issue", WebURL: "https://gitlab.com/OWNER/REPO/-/issues/1"}, nil, nil
		})

	exec := cmdtest.SetupCmdForTest(t, NewCmdCreate, false, cmdtest.WithGitLabClient(testClient.Client))

	output, err := exec(`--title "Test Issue" --template bug --yes`)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/-/issues/1")
}

func TestIssueCreate_TemplateWithMdExtension(t *testing.T) {
	// Cannot use t.Parallel(): test mutates git.ToplevelDir, a package-level variable.
	t.Setenv("NO_COLOR", "true")

	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, ".gitlab", "issue_templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "bug.md"), []byte("## Bug Report\n\nDescribe the bug here."), 0o644))

	origToplevelDir := git.ToplevelDir
	git.ToplevelDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { git.ToplevelDir = origToplevelDir })

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                37777023,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
		}, nil, nil)
	testClient.MockIssues.EXPECT().
		CreateIssue("OWNER/REPO", gomock.Any()).
		DoAndReturn(func(_ any, opts *gitlab.CreateIssueOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.Issue, *gitlab.Response, error) {
			assert.Equal(t, "## Bug Report\n\nDescribe the bug here.", *opts.Description)
			return &gitlab.Issue{IID: 1, Title: "Test Issue", WebURL: "https://gitlab.com/OWNER/REPO/-/issues/1"}, nil, nil
		})

	exec := cmdtest.SetupCmdForTest(t, NewCmdCreate, false, cmdtest.WithGitLabClient(testClient.Client))

	// Passing "bug.md" should work the same as "bug"
	output, err := exec(`--title "Test Issue" --template bug.md --yes`)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/-/issues/1")
}

func TestIssueCreate_TemplateNotFound(t *testing.T) {
	// Cannot use t.Parallel(): test mutates git.ToplevelDir, a package-level variable.
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".gitlab", "issue_templates"), 0o755))

	origToplevelDir := git.ToplevelDir
	git.ToplevelDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { git.ToplevelDir = origToplevelDir })

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                37777023,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
		}, nil, nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdCreate, false, cmdtest.WithGitLabClient(testClient.Client))

	_, err := exec(`--title "Test Issue" --template nonexistent --yes`)
	assert.ErrorContains(t, err, `template "nonexistent" not found in .gitlab/issue_templates/`)
}

func TestIssueCreate_TemplateMutuallyExclusiveWithDescription(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmdCreate, false)

	_, err := exec(`--title "Test Issue" --template bug --description "my description"`)
	assert.Error(t, err)
}

func TestIssueCreateWhenIssuesDisabled(t *testing.T) {
	// GIVEN
	testClient := gitlabtesting.NewTestClient(t)

	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                37777023,
			Description:       "this is a test description",
			Name:              "REPO",
			NameWithNamespace: "Test User / REPO",
			Path:              "REPO",
			PathWithNamespace: "OWNER/REPO",
			DefaultBranch:     "main",
			HTTPURLToRepo:     "https://gitlab.com/OWNER/REPO.git",
			WebURL:            "https://gitlab.com/OWNER/REPO",
			ReadmeURL:         "https://gitlab.com/OWNER/REPO/-/blob/main/README.md",
			IssuesEnabled:     false,
		}, nil, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdCreate,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	// WHEN
	cli := `--title "test title" --description "test description"`
	output, err := exec(cli)

	// THEN
	assert.NotNil(t, err)
	assert.Empty(t, output.String())
	assert.Equal(t, "Issues are disabled for project \"OWNER/REPO\" or require project membership. "+
		"Make sure issues are enabled for the \"OWNER/REPO\" project, and if required, you are a member of the project.\n",
		output.Stderr())
}
