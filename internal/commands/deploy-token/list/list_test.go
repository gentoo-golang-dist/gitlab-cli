//go:build !integration

package list

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
	cmd := NewCmdList(factory)
	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

var projectDeployTokenResponse = heredoc.Doc(`
[
  {
    "id": 12,
    "name": "test dt",
    "username": "gitlab+deploy-token-12",
    "expires_at": null,
    "scopes": [
      "read_repository"
    ],
    "revoked": true,
    "expired": false
  }
]
`)

func TestListProjectDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))
	output, err := runCommand(t, fakeHTTP, "")
	if err != nil {
		t.Errorf("error running command `token list`: %v", err)
	}

	out := output.String()

	assert.Equal(t, heredoc.Doc(`
	ID  NAME    USERNAME               EXPIRES_AT REVOKED  EXPIRED  SCOPES         
	12  test dt gitlab+deploy-token-12 -          true     false    read_repository
	`), out)
	assert.Empty(t, output.Stderr())
}

func TestListProjectDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))
	output, err := runCommand(t, fakeHTTP, "--output json")
	if err != nil {
		t.Errorf("error running command `token list --output json`: %v", err)
	}
	assert.Empty(t, output.Stderr())
	assert.JSONEq(t, projectDeployTokenResponse, output.String())
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

func TestListGroupDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))
	output, err := runCommand(t, fakeHTTP, "--group GROUP")
	if err != nil {
		t.Errorf("error running command `token list --group GROUP`: %v", err)
	}

	out := output.String()

	assert.Equal(t, heredoc.Doc(`
	ID  NAME  USERNAME  EXPIRES_AT REVOKED  EXPIRED  SCOPES         
	19  toto  test-mydt -          false    false    read_repository
	`), out)
	assert.Empty(t, output.Stderr())
}

func TestListGroupDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))

	output, err := runCommand(t, fakeHTTP, "--group GROUP --output json")
	if err != nil {
		t.Errorf("error running command `token list --group GROUP --output json`: %v", err)
	}
	assert.Empty(t, output.Stderr())
	assert.JSONEq(t, groupDeployTokenResponse, output.String())
}
