//go:build !integration

package run_trig

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

type ResponseJSON struct {
	Token string `json:"token"`
	Ref   string `json:"ref"`
}

func runCommand(t *testing.T, rt http.RoundTripper, cli string) (*test.CmdOut, error) {
	t.Helper()

	ios, _, stdout, stderr := cmdtest.TestIOStreams()
	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname).Lab()),
	)

	factory.BranchStub = func() (string, error) {
		return "custom-branch-123", nil
	}

	cmd := NewCmdRunTrig(factory)

	return cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)
}

func TestCIRun(t *testing.T) {
	tests := []struct {
		name             string
		cli              string
		ciJobToken       string
		expectedPOSTBody string
		expectedOut      string
	}{
		{
			name:             "when running `ci run-trig` without branch parameter, defaults to current branch",
			cli:              "-t foobar",
			ciJobToken:       "",
			expectedPOSTBody: `"ref":"custom-branch-123"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: custom-branch-123, weburl: https://gitlab.com/OWNER/REPO/-/pipelines/123\n",
		},
		{
			name:             "when running `ci run-trig` with branch parameter, run CI at branch",
			cli:              "-t foobar -b ci-cd-improvement-399",
			ciJobToken:       "",
			expectedPOSTBody: `"ref":"ci-cd-improvement-399"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: ci-cd-improvement-399, weburl: https://gitlab.com/OWNER/REPO/-/pipelines/123\n",
		},
		{
			name:             "when running `ci run-trig` without any parameter, takes trigger token from env variable",
			cli:              "",
			ciJobToken:       "foobar",
			expectedPOSTBody: `"ref":"custom-branch-123"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: custom-branch-123, weburl: https://gitlab.com/OWNER/REPO/-/pipelines/123\n",
		},
		{
			name:             "when running `ci run-trig` with input parameter, run CI with untyped input",
			cli:              "-t foobar -i key1:val1 --input key2:val2",
			expectedPOSTBody: `"ref":"custom-branch-123","token":"foobar","inputs":{"key1":"val1","key2":"val2"}`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: custom-branch-123, weburl: https://gitlab.com/OWNER/REPO/-/pipelines/123\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHTTP := &httpmock.Mocker{
				MatchURL: httpmock.PathAndQuerystring,
			}
			defer fakeHTTP.Verify(t)

			t.Setenv("CI_JOB_TOKEN", tc.ciJobToken)

			fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/projects/OWNER/REPO/trigger/pipeline",
				func(req *http.Request) (*http.Response, error) {
					rb, _ := io.ReadAll(req.Body)

					var response ResponseJSON
					err := json.Unmarshal(rb, &response)
					if err != nil {
						fmt.Printf("Error when parsing response body %s\n", rb)
					}

					if response.Token != "foobar" {
						fmt.Printf("Invalid token %s\n", rb)
					}

					// ensure CLI runs CI on correct branch
					assert.Contains(t, string(rb), tc.expectedPOSTBody)
					resp, _ := httpmock.NewStringResponse(http.StatusOK, fmt.Sprintf(`{
 						"id": 123,
 						"iid": 123,
 						"project_id": 3,
 						"status": "created",
 						"ref": "%s",
						"web_url": "https://gitlab.com/OWNER/REPO/-/pipelines/123"}`, response.Ref))(req)
					return resp, nil
				},
			)

			output, _ := runCommand(t, fakeHTTP, tc.cli)

			out := output.String()

			assert.Equal(t, tc.expectedOut, out)
			assert.Empty(t, output.Stderr())
		})
	}
}

func runTrigCommandWithRepoOverride(t *testing.T, rt http.RoundTripper, cli string) (*test.CmdOut, error) {
	t.Helper()
	ios, _, stdout, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
	factory := cmdtest.NewTestFactory(ios,
		cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, &http.Client{Transport: rt}, "", glinstance.DefaultHostname).Lab()),
		cmdtest.WithBaseRepo("OTHER", "TARGET", glinstance.DefaultHostname),
	)

	// This simulates being in a local git repo with a different branch name
	factory.BranchStub = func() (string, error) {
		return "feature-branch-not-in-target", nil
	}

	cmd := NewCmdRunTrig(factory)
	// Manually add the repo flag since we're testing in isolation
	cmd.Flags().StringP("repo", "R", "", "Select another repository using the OWNER/REPO format or the project ID. Supports group namespaces.")

	cmdOut, err := cmdtest.ExecuteCommand(cmd, cli, stdout, stderr)

	return cmdOut, err
}

func TestCIRunTrigRepoOverride(t *testing.T) {
	tests := []struct {
		name string
		cli  string

		expectedPOSTBody string
		expectedOut      string
		repoOverride     bool
	}{
		{
			name:             "when running `ci run-trig` without repo override, uses current branch",
			cli:              "-t token123",
			expectedPOSTBody: `"ref":"feature-branch-not-in-target"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: feature-branch-not-in-target, weburl: https://gitlab.com/OTHER/TARGET/-/pipelines/123\n",
			repoOverride:     false,
		},
		{
			name:             "when running `ci run-trig` with repo override but no branch flag, uses target repo default branch",
			cli:              "-R OTHER/TARGET -t token123",
			expectedPOSTBody: `"ref":"main"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: main, weburl: https://gitlab.com/OTHER/TARGET/-/pipelines/123\n",
			repoOverride:     true,
		},
		{
			name:             "when running `ci run-trig` with repo override and explicit branch, uses explicit branch",
			cli:              "-R OTHER/TARGET -b develop -t token123",
			expectedPOSTBody: `"ref":"develop"`,
			expectedOut:      "Created pipeline (ID: 123), status: created, ref: develop, weburl: https://gitlab.com/OTHER/TARGET/-/pipelines/123\n",
			repoOverride:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHTTP := &httpmock.Mocker{
				MatchURL: httpmock.PathAndQuerystring,
			}
			defer fakeHTTP.Verify(t)

			// Mock getting the default branch for repo override
			if tc.repoOverride && tc.cli != "-R OTHER/TARGET -b develop -t token123" {
				// Only mock the API call when we actually need to get the default branch
				fakeHTTP.RegisterResponder(http.MethodGet, "/api/v4/projects/OTHER/TARGET",
					httpmock.NewStringResponse(http.StatusOK, `{
						"id": 42,
						"name": "TARGET",
						"default_branch": "main"
					}`))
			}

			fakeHTTP.RegisterResponder(http.MethodPost, "/api/v4/projects/OTHER/TARGET/trigger/pipeline",
				func(req *http.Request) (*http.Response, error) {
					rb, _ := io.ReadAll(req.Body)

					var response map[string]interface{}
					err := json.Unmarshal(rb, &response)
					if err != nil {
						fmt.Printf("Error when parsing response body %s\n", rb)
					}

					// Ensure CLI runs CI on correct branch
					assert.Contains(t, string(rb), tc.expectedPOSTBody)

					ref := response["ref"].(string)
					resp, _ := httpmock.NewStringResponse(http.StatusOK, fmt.Sprintf(`{
						"id": 123,
						"status": "created",
						"ref": "%s",
						"web_url": "https://gitlab.com/OTHER/TARGET/-/pipelines/123"
					}`, ref))(req)
					return resp, nil
				},
			)

			output, err := runTrigCommandWithRepoOverride(t, fakeHTTP, tc.cli)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			assert.Equal(t, tc.expectedOut, output.String())
		})
	}
}
