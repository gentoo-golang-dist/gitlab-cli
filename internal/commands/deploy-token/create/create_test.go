//go:build !integration

package create

import (
	"encoding/json"
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
	cmd := NewCmdCreate(factory)
	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

var groupDeployTokenResponse = heredoc.Doc(`
{
  "id": 21,
  "name": "my-group-token",
  "username": "group-dt",
  "expires_at": null,
  "scopes": [
    "read_repository",
    "read_registry"
  ],
  "revoked": false,
  "expired": false,
  "token": "gldt-helloWorldHowAreYouu"
}

`)

func TestCreateGroupDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, "[]"))
	fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))

	output, err := runCommand(t, fakeHTTP, "--group GROUP --output json --scope read_registry --scope read_repository --username group-dt my-group-token")
	if err != nil {
		t.Error(err)
		return
	}

	var expect any
	var actual any

	if err := json.Unmarshal([]byte(groupDeployTokenResponse), &expect); err != nil {
		t.Error(err)
	}

	if err := json.Unmarshal([]byte(output.String()), &actual); err != nil {
		t.Error(err)
	}
	assert.Equal(t, expect, actual)
	assert.Empty(t, output.Stderr())
}

func TestCreateGroupDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, "[]"))
	fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/groups/GROUP/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, groupDeployTokenResponse))

	output, err := runCommand(t, fakeHTTP, "--group GROUP --output text --scope read_registry --scope read_repository my-group-token")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, "gldt-helloWorldHowAreYouu\n", output.String())
}

var projectDeployTokenResponse = heredoc.Doc(`
{
  "id": 22,
  "name": "my-project-token",
  "username": "gitlab+deploy-token-22",
  "expires_at": null,
  "scopes": [
    "read_repository",
    "read_registry"
  ],
  "revoked": false,
  "expired": false,
  "token": "gldt-WhoReadsTheseTokensZ"
}

`)

func TestCreateProjectDeployTokenAsJSON(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, "[]"))
	fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))

	output, err := runCommand(t, fakeHTTP, "--output json --scope read_repository --scope read-registry my-project-token")
	if err != nil {
		t.Error(err)
		return
	}

	var expect any
	var actual any

	if err := json.Unmarshal([]byte(projectDeployTokenResponse), &expect); err != nil {
		t.Error(err)
	}

	if err := json.Unmarshal([]byte(output.String()), &actual); err != nil {
		t.Error(err)
	}
	assert.Equal(t, expect, actual)
	assert.Empty(t, output.Stderr())
}

func TestCreateProjectDeployTokenAsText(t *testing.T) {
	fakeHTTP := &httpmock.Mocker{}
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, "[]"))
	fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/projects/OWNER/REPO/deploy_tokens",
		httpmock.NewStringResponse(http.StatusOK, projectDeployTokenResponse))

	output, err := runCommand(t, fakeHTTP, "--output text --scope read_repository --scope api my-project-token")
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, "gldt-WhoReadsTheseTokensZ\n", output.String())
}
