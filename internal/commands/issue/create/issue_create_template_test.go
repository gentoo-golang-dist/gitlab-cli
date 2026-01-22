//go:build !integration

package create

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestIssueCreate_WithTemplate(t *testing.T) {
	// 1. Mock LoadGitLabTemplate
	oldLoadTemplate := cmdutils.LoadGitLabTemplate
	defer func() { cmdutils.LoadGitLabTemplate = oldLoadTemplate }()
	cmdutils.LoadGitLabTemplate = func(tmplType, tmplName string) (string, error) {
		if tmplName == "bug" {
			return "Template Description", nil
		}
		return "", fmt.Errorf("template not found: %s", tmplName)
	}

	// 2. Mock createIssue to capture options
	var createdOpts *gitlab.CreateIssueOptions
	oldCreateIssue := createIssue
	defer func() { createIssue = oldCreateIssue }()
	createIssue = func(client *gitlab.Client, projectID any, opts *gitlab.CreateIssueOptions) (*gitlab.Issue, error) {
		createdOpts = opts
		now := time.Now()
		return &gitlab.Issue{
			IID:         1,
			Title:       *opts.Title,
			Description: *opts.Description,
			WebURL:      "https://gitlab.com/OWNER/REPO/-/issues/1",
			CreatedAt:   &now,
			State:       "opened",
		}, nil
	}

	// 3. Setup Command
	testClient := gitlabtesting.NewTestClient(t)

	// mock GetProject
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                1,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
			WebURL:            "https://gitlab.com/OWNER/REPO",
		}, nil, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdCreate,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	// 4. Run Command
	output, err := exec(`--title "My Issue" --template "bug"`)

	// 5. Assertions
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/-/issues/1")

	assert.NotNil(t, createdOpts)
	if createdOpts != nil {
		assert.Equal(t, "My Issue", *createdOpts.Title)
		assert.Equal(t, "Template Description", *createdOpts.Description)
	}
}

func TestIssueCreate_TemplateNotFound(t *testing.T) {
	// 1. Mock LoadGitLabTemplate to return empty string (template not found)
	oldLoadTemplate := cmdutils.LoadGitLabTemplate
	defer func() { cmdutils.LoadGitLabTemplate = oldLoadTemplate }()
	cmdutils.LoadGitLabTemplate = func(tmplType, tmplName string) (string, error) {
		return "", nil // empty string means template not found
	}

	// 2. Setup Command
	testClient := gitlabtesting.NewTestClient(t)

	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                1,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
		}, nil, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdCreate,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	// 3. Run Command
	_, err := exec(`--title "My Issue" --template "nonexistent"`)

	// 4. Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `template "nonexistent" not found or empty`)
}

func TestIssueCreate_TemplateAndDescriptionError(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)

	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{
			ID:                1,
			PathWithNamespace: "OWNER/REPO",
			IssuesEnabled:     true,
		}, nil, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmdCreate,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	_, err := exec(`--title "My Issue" --template "bug" --description "Manual Description"`)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both --template and --description")
}
