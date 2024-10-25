package note

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/cli/commands/cmdtest"
	"gitlab.com/gitlab-org/cli/pkg/git"
	"gitlab.com/gitlab-org/cli/pkg/httpmock"
	"gitlab.com/gitlab-org/cli/pkg/prompt"
	"gitlab.com/gitlab-org/cli/test"
)

func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "mr_note_update_test")
}

func runCommand(rt http.RoundTripper, isTTY bool, cli string) (*test.CmdOut, error) {
	ios, _, stdout, stderr := cmdtest.InitIOStreams(isTTY, "")
	factory := cmdtest.InitFactory(ios, rt)
	factory.Branch = git.CurrentBranch

	// TODO: shouldn't be there but the stub doesn't work without it
	_, _ = factory.HttpClient()

	cmd := UpdateCmdNote(factory)

	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

func Test_UpdateCmdNote(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("--message flag specified", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodPut, "/projects/OWNER/REPO/merge_requests/1/notes/301",
			httpmock.NewStringResponse(http.StatusOK, `
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
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))
		// glab mr note update 1 301 --message "Here is my note"
		output, err := runCommand(fakeHTTP, true, `1 301 --message "Here is my note"`)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n")
	})
}

func Test_UpdateCmdNote_error(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("note does not exist", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodPut, "/projects/OWNER/REPO/merge_requests/1/notes/301",
			httpmock.NewStringResponse(http.StatusNotFound, `
		{
			"message": "Not Found"
		}
	`))

		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
		{
  			"id": 1,
  			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))

		// glab mr note update 1 301 --message "Here is my note"
		_, err := runCommand(fakeHTTP, true, `1 301 --message "Some message"`)
		assert.NotNil(t, err)
		assert.Equal(t, "404 Not Found", err.Error())
	})

	t.Run("note could not be updated", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodPut, "/projects/OWNER/REPO/merge_requests/1/notes/301",
			httpmock.NewStringResponse(http.StatusUnauthorized, `
		{
			"message": "note not found"
		}
	`))
		fakeHTTP.RegisterResponder(http.MethodGet, "/projects/OWNER/REPO/merge_requests/1",
			httpmock.NewStringResponse(http.StatusOK, `
		{
			"id": 1,
			"iid": 1,
			"web_url": "https://gitlab.com/OWNER/REPO/merge_requests/1"
		}
	`))
		// glab mr note 1 301 --message "Here is my note"
		_, err := runCommand(fakeHTTP, true, `1 301 --message "Here is my note"`)
		assert.NotNil(t, err)
		assert.Equal(t, "PUT https://gitlab.com/api/v4/projects/OWNER/REPO/merge_requests/1/notes/301: 401 {message: note not found}", err.Error())
	})
}

func Test_mrNoteCreate_prompt(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	t.Run("message provided", func(t *testing.T) {
		fakeHTTP.RegisterResponder(http.MethodPut, "/projects/OWNER/REPO/merge_requests/1/notes/301",
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
		as.StubOne("some note message")

		// glab mr note update 1
		output, err := runCommand(fakeHTTP, true, `1 301`)
		if err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, output.Stderr(), "")
		assert.Equal(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n")
	})

	t.Run("message is empty", func(t *testing.T) {
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

		// glab mr note update 1
		_, err := runCommand(fakeHTTP, true, `1 301`)
		if err == nil {
			t.Error("expected error")
			return
		}
		assert.Equal(t, err.Error(), "aborted... Note has an empty message.")
	})
}
