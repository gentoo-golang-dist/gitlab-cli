package comment

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/prompt"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "mr_comment_test")
}

func runCommand(t *testing.T, rt http.RoundTripper, cli string) (*test.CmdOut, error) {
	t.Helper()

	ios, _, stdout, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))

	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname).Lab()),
	)
	factory.BranchStub = git.CurrentBranch

	cmd := NewCmdComment(factory)

	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

func Test_NewCmdComment(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("list comments", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1/notes",
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

		output, err := runCommand(t, fakeHTTP, `list 1`)
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
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/merge_requests/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "MergeRequest",
				"noteable_iid": 1
			}
		`))

		output, err := runCommand(t, fakeHTTP, `add 1 --message "Here is my comment"`)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n")
	})

	t.Run("add comment with prompt", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/merge_requests/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "MergeRequest",
				"noteable_iid": 1
			}
		`))

		as, teardown := prompt.InitAskStubber()
		defer teardown()
		as.StubOne("some comment message")

		output, err := runCommand(t, fakeHTTP, `add 1`)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n")
	})

	t.Run("reply to comment", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1",
				"author": {"username": "original_author"}
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/merge_requests/1/notes",
			httpmock.NewStringResponse(http.StatusCreated, `
			{
				"id": 301,
				"created_at": "2013-10-02T08:57:14Z",
				"updated_at": "2013-10-02T08:57:14Z",
				"system": false,
				"noteable_id": 1,
				"noteable_type": "MergeRequest",
				"noteable_iid": 1
			}
		`))

		output, err := runCommand(t, fakeHTTP, `reply 1 123 --message "Thanks for the feedback"`)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n")
	})

	t.Run("merge request not found", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/122",
			httpmock.NewStringResponse(http.StatusNotFound, `
			{
				"message": "merge request not found"
			}
		`))

		_, err := runCommand(t, fakeHTTP, `list 122`)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "404 Not Found")
	})

	t.Run("empty message error", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		as, teardown := prompt.InitAskStubber()
		defer teardown()
		as.StubOne("")

		_, err := runCommand(t, fakeHTTP, `add 1`)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "aborted... Comment has an empty message")
	})

	t.Run("invalid comment ID for reply", func(t *testing.T) {
		_, err := runCommand(t, fakeHTTP, `reply 1 invalid --message "test"`)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID")
	})
}

func Test_NewCmdComment_error(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("comment could not be created", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodPost, "/projects/OWNER/REPO/merge_requests/1/notes",
			httpmock.NewStringResponse(http.StatusUnauthorized, `
			{
				"message": "Unauthorized"
			}
		`))

		_, err := runCommand(t, fakeHTTP, `add 1 --message "test"`)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("comments could not be listed", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
			{
				"id": 1,
				"iid": 1,
				"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
			}
		`))

		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1/notes",
			httpmock.NewStringResponse(http.StatusUnauthorized, `
			{
				"message": "Unauthorized"
			}
		`))

		_, err := runCommand(t, fakeHTTP, `list 1`)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

func Test_NewCmdComment_json_output(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
		httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1/notes",
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

	output, err := runCommand(t, fakeHTTP, `list 1 --output json`)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, output.Stderr(), "")
	assert.Contains(t, output.String(), `"id":301`)
	assert.Contains(t, output.String(), `"body":"This is a comment"`)
}

func Test_NewCmdComment_system_logs(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
		httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1/notes",
		httpmock.NewStringResponse(http.StatusOK, `
		[
			{
				"id": 301,
				"body": "This is a regular comment",
				"author": {"username": "user1"},
				"created_at": "2013-10-02T08:57:14Z",
				"system": false
			},
			{
				"id": 302,
				"body": "System activity",
				"author": {"username": "system"},
				"created_at": "2013-10-02T09:00:00Z",
				"system": true
			}
		]
	`))

	// Test without system logs
	output, err := runCommand(t, fakeHTTP, `list 1`)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, output.Stderr(), "")
	assert.Contains(t, output.String(), "This is a regular comment")
	assert.NotContains(t, output.String(), "System activity")

	// Test with system logs - need to register the MR endpoint again
	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
		httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))

	fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1/notes",
		httpmock.NewStringResponse(http.StatusOK, `
		[
			{
				"id": 301,
				"body": "This is a regular comment",
				"author": {"username": "user1"},
				"created_at": "2013-10-02T08:57:14Z",
				"system": false
			},
			{
				"id": 302,
				"body": "System activity",
				"author": {"username": "system"},
				"created_at": "2013-10-02T09:00:00Z",
				"system": true
			}
		]
	`))

	output, err = runCommand(t, fakeHTTP, `list 1 --system-logs`)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, output.Stderr(), "")
	assert.Contains(t, output.String(), "This is a regular comment")
	assert.Contains(t, output.String(), "System activity")
}
