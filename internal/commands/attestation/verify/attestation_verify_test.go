//go:build !integration

package verify

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

func runCommand(t *testing.T, rt http.RoundTripper, artifactPath string) (*test.CmdOut, error) {
	t.Helper()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()
	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname).Lab()),
	)
	cmd := NewCmdVerify(factory)
	cmd.Flags().Set("project", "OWNER/REPO")
	return cmdtest.ExecuteCommand(cmd, artifactPath, stdout, stderr)
}

func Test_AttestationVerify(t *testing.T) {
	fakeHTTP := httpmock.New()
	defer fakeHTTP.Verify(t)

	fakeHTTP.RegisterResponder(http.MethodGet, `/projects/OWNER/REPO`,
		httpmock.NewFileResponse(http.StatusOK, "./testdata/project.json"))

	fakeHTTP.RegisterResponder(http.MethodGet, `/projects/OWNER/REPO/attestations/f2d4bc357309c633154f1e94c6fda3583ae429f6adc882d4d9006380ea3a79da`,
		httpmock.NewFileResponse(http.StatusOK, "./testdata/attestationsList.json"))

	fakeHTTP.RegisterResponder(http.MethodGet, `/projects/OWNER/REPO/attestations/1/download`,
		httpmock.NewFileResponse(http.StatusOK, "./testdata/attestationDownload.json"))

	artifactPath := "testdata/example_artifact.txt"
	output, err := runCommand(t, fakeHTTP, artifactPath)
	
	// This is the latest point at which we can test without hitting Sigstore infrastructure
	expectedErrorMsg := "failed to verify signature: provided artifact digests does not match digests in statement"
	if assert.EqualErrorf(t, err, expectedErrorMsg, "Error should be: %v, got: %v", expectedErrorMsg, err) {
		assert.Empty(t, output.Stderr())
	}

}
