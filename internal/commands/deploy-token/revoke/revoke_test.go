//go:build !integration

package revoke

import (
	"net/http"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

func runCommand(t *testing.T, rt http.RoundTripper, cli string) (*test.CmdOut, error) {
	t.Helper()

	ios, _, stdout, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname)),
	)
	cmd := NewCmdRevoke(factory)
	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

var groupDeployTokenResponse = heredoc.Doc(`
[
  {
    "id": 19,
    "name": "toto",
    "username": "test-mydt",
    "expires_at": null,
    "scopes": [
      "read_repository"
    ],
    "revoked": false,
    "expired": false
  }
]
`)

func TestRevokeGroupDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))
	fakeHTTP.RegisterResponder(http.MethodDelete, "/api/v4/groups/GROUP/deploy_tokens/19",
		httpmock.NewStringResponse(http.StatusOK, ""))

	output, err := runCommand(t, fakeHTTP, "--group GROUP toto --output json")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Empty(t, output.Stderr())
}

func TestRevokeGroupDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))
	fakeHTTP.RegisterResponder(http.MethodDelete, "/api/v4/groups/GROUP/deploy_tokens/19",
		httpmock.NewStringResponse(http.StatusOK, ""))

	output, err := runCommand(t, fakeHTTP, "--group GROUP toto")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, "revoked toto 19", output.String())
}

var projectDeployTokenResponse = heredoc.Doc(`
[
  {
    "id": 20,
    "name": "my-project-dt",
    "username": "group-dt",
    "expires_at": null,
    "scopes": [
      "read_repository"
    ],
    "revoked": false,
    "expired": false
  }
]`)

func TestRevokeProjectDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))
	fakeHTTP.RegisterResponder(http.MethodDelete, "/api/v4/projects/OWNER/REPO/deploy_tokens/20",
		httpmock.NewStringResponse(http.StatusOK, ""))

	output, err := runCommand(t, fakeHTTP, "--output json my-project-dt")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Empty(t, output.Stderr())
}

func TestRevokeProjectDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))
	fakeHTTP.RegisterResponder(http.MethodDelete, "/api/v4/projects/OWNER/REPO/deploy_tokens/20",
		httpmock.NewStringResponse(http.StatusOK, ""))

	output, err := runCommand(t, fakeHTTP, "my-project-dt")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, "revoked my-project-dt 20", output.String())
}
