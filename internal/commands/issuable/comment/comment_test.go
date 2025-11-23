package comment

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/cli/internal/commands/issuable"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/prompt"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

func runCommand(t *testing.T, rt http.RoundTripper, cli string, issueType issuable.IssueType) (*test.CmdOut, error) {
	t.Helper()

	ios, _, stdout, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname).Lab()),
		cmdtest.WithConfig(config.NewFromString("editor: vi")),
	)

	cmd := NewCmdComment(factory)
	// Set the issueType annotation so determineIssueType can find it
	cmd.Annotations = map[string]string{
		"issueType": string(issueType),
	}

	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

func Test_NewCmdComment_Issue(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("list comments", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "issue",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1/notes",
			httpmock.NewStringResponse(http.StatusOK, `
			[
				{
					"id": 301,
					"body": "This is a comment",
					"author": {"username": "user1"},
					"created_at": "2013-10-02T08:57:14Z",
					"system": false
				},
				{
					"id": 302,
					"body": "This is another comment",
					"author": {"username": "user2"},
					"created_at": "2013-10-02T09:00:00Z",
					"system": false
				}
			]
		`))

		output, err := runCommand(t, fakeHTTP, `list 1`, issuable.TypeIssue)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Contains(t, output.String(), "Comments / Notes")
		assert.Contains(t, output.String(), "user1")
		assert.Contains(t, output.String(), "user2")
		assert.Contains(t, output.String(), "This is a comment")
		assert.Contains(t, output.String(), "This is another comment")
	})

	t.Run("add comment with message flag", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "issue",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/issues/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "Issue",
				"noteable_iid": 1
			}
		`))

		output, err := runCommand(t, fakeHTTP, `add 1 --message "Here is my comment"`, issuable.TypeIssue)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/issues/1#note_301\n")
	})

	t.Run("add comment with prompt", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "issue",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/issues/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "Issue",
				"noteable_iid": 1
			}
		`))

		as, teardown := prompt.InitAskStubber()
		defer teardown()
		as.StubOne("some comment message")

		output, err := runCommand(t, fakeHTTP, `add 1`, issuable.TypeIssue)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/issues/1#note_301\n")
	})

	t.Run("reply to comment", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "issue",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1",
				"author": {"username": "original_author"}
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/issues/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "Issue",
				"noteable_iid": 1
			}
		`))

		output, err := runCommand(t, fakeHTTP, `reply 1 123 --message "Thanks for the feedback"`, issuable.TypeIssue)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/issues/1#note_301\n")
	})

	t.Run("issue not found", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/122",
			httpmock.NewStringResponse(http.StatusNotFound, `
			{
				"message": "issue not found"
			}
		`))

		_, err := runCommand(t, fakeHTTP, `list 122`, issuable.TypeIssue)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "404 Not Found")
	})

	t.Run("empty message error", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "issue",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
			}
		`))

		as, teardown := prompt.InitAskStubber()
		defer teardown()
		as.StubOne("")

		_, err := runCommand(t, fakeHTTP, `add 1`, issuable.TypeIssue)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "aborted... Comment has an empty message")
	})

	t.Run("invalid comment ID for reply", func(t *testing.T) {
		_, err := runCommand(t, fakeHTTP, `reply 1 invalid --message "test"`, issuable.TypeIssue)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID")
	})
}

func Test_NewCmdComment_Incident(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("list comments", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"issue_type": "incident",
				"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1/notes",
			httpmock.NewStringResponse(http.StatusOK, `
			[
				{
					"id": 301,
					"body": "This is a comment",
					"author": {"username": "user1"},
					"created_at": "2013-10-02T08:57:14Z",
					"system": false
				}
			]
		`))

		output, err := runCommand(t, fakeHTTP, `list 1`, issuable.TypeIncident)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Contains(t, output.String(), "Comments / Notes")
		assert.Contains(t, output.String(), "user1")
		assert.Contains(t, output.String(), "This is a comment")
	})
}

func Test_NewCmdComment_error(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	commands := []struct {
		name      string
		issueType issuable.IssueType
	}{
		{"issue", issuable.TypeIssue},
		{"incident", issuable.TypeIncident},
	}

	for _, cc := range commands {
		t.Run("comment could not be created", func(t *testing.T) {
			fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
				httpmock.NewStringResponse(http.StatusOK, fmt.Sprintf(`
				{
					"id": 1,
					"iid": 1,
					"issue_type": "%s",
					"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
				}
			`, cc.issueType)))

			fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/issues/1/notes",
				httpmock.NewStringResponse(http.StatusUnauthorized, `
				{
					"message": "Unauthorized"
				}
			`))

			_, err := runCommand(t, fakeHTTP, `add 1 --message "test"`, cc.issueType)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "401")
		})

		t.Run("comments could not be listed", func(t *testing.T) {
			fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
				httpmock.NewStringResponse(http.StatusOK, fmt.Sprintf(`
				{
					"id": 1,
					"iid": 1,
					"issue_type": "%s",
					"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
				}
			`, cc.issueType)))

			fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1/notes",
				httpmock.NewStringResponse(http.StatusUnauthorized, `
				{
					"message": "Unauthorized"
				}
			`))

			_, err := runCommand(t, fakeHTTP, `list 1`, cc.issueType)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "401")
		})
	}
}

func Test_NewCmdComment_json_output(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1",
		httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"issue_type": "issue",
			"web_url": "https://gitlab.com/OWNER/REPO/issues/1"
		}
	`))

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/issues/1/notes",
		httpmock.NewStringResponse(http.StatusOK, `
		[
			{
				"id": 301,
				"body": "This is a comment",
				"author": {"username": "user1"},
				"created_at": "2013-10-02T08:57:14Z",
				"system": false
			}
		]
	`))

	output, err := runCommand(t, fakeHTTP, `list 1 --output json`, issuable.TypeIssue)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, output.Stderr(), "")
	assert.Contains(t, output.String(), `"id":301`)
	assert.Contains(t, output.String(), `"body":"This is a comment"`)
}
