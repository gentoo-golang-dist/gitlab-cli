package checkout

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/pkg/git"
	"gitlab.com/gitlab-org/cli/pkg/httpmock"
	"gitlab.com/gitlab-org/cli/test"
)

func runCommand(rt http.RoundTripper, branch string, isTTY bool, cli string) (*test.CmdOut, error) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	ios, _, _, _ := cmdtest.InitIOStreams(isTTY, "")

	pu, _ := url.Parse("https://gitlab.com/OWNER/REPO.git")

	factory := cmdtest.InitFactory(ios, rt)

	factory.Remotes = func() (glrepo.Remotes, error) {
		return glrepo.Remotes{
			{
				Remote: &git.Remote{
					Name:     "upstream",
					Resolved: "base",
					PushURL:  pu,
				},
				Repo: glrepo.New("OWNER", "REPO"),
			},
			{
				Remote: &git.Remote{
					Name:     "origin",
					Resolved: "base",
					PushURL:  pu,
				},
				Repo: glrepo.New("monalisa", "REPO"),
			},
		}, nil
	}

	factory.Branch = func() (string, error) {
		return branch, nil
	}

	_, _ = factory.HttpClient()

	cmd := NewCmdCheckout(factory)

	// Set on root cmd, thus we also need to set it here.
	cmd.SilenceUsage = true

	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	args := strings.Fields(cli)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return &test.CmdOut{
		OutBuf: outBuf,
		ErrBuf: errBuf,
	}, err
}

func TestMrCheckout(t *testing.T) {
	type httpMock struct {
		method string
		path   string
		status int
		body   string
	}

	tests := []struct {
		name              string
		commandArgs       string
		branch            string
		httpMocks         []httpMock
		shelloutStubs     []string
		expectedShellouts []string
		expectedError     bool
		expectedOutput    string
	}{
		{
			name:        "when a valid MR is checked out using MR id",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"oldcommit\n",
				"oldcommit\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git fetch git@gitlab.com:OWNER/REPO.git +refs/heads/feat-new-mr:feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
				"git config branch.feat-new-mr.remote git@gitlab.com:OWNER/REPO.git",
				"git config branch.feat-new-mr.merge refs/heads/feat-new-mr",
				"git checkout feat-new-mr",
			},
			expectedOutput: "✓ Branch 'feat-new-mr' is up to date with the merge request.\n",
		},
		{
			name:        "when a valid MR is checked out with HTTPS remote",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"https://gitlab.com/OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"oldcommit\n",
				"oldcommit\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch https://gitlab.com/OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git fetch https://gitlab.com/OWNER/REPO.git +refs/heads/feat-new-mr:feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
				"git config branch.feat-new-mr.remote https://gitlab.com/OWNER/REPO.git",
				"git config branch.feat-new-mr.merge refs/heads/feat-new-mr",
				"git checkout feat-new-mr",
			},
			expectedOutput: "✓ Branch 'feat-new-mr' is up to date with the merge request.\n",
		},
		{
			name:        "when a valid MR comes from a forked private project",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"target_project_id": 4,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/4",
					status: http.StatusOK,
					body: `{
						"id": 4,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusNotFound,
					body: `{
						"message":"404 Project Not Found"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"oldcommit\n",
				"oldcommit\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/merge-requests/123/head:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git fetch git@gitlab.com:OWNER/REPO.git +refs/merge-requests/123/head:feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
				"git config branch.feat-new-mr.remote git@gitlab.com:OWNER/REPO.git",
				"git config branch.feat-new-mr.merge refs/merge-requests/123/head",
				"git checkout feat-new-mr",
			},
			expectedOutput: "✓ Branch 'feat-new-mr' is up to date with the merge request.\n",
		},
		{
			name:        "when a valid MR is checked out using MR id and specifying branch",
			commandArgs: "123 --branch foo",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 4,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": true,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/4",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:FORK_OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/FORK_OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"https://gitlab.com/OWNER/REPO.git\n",
				"refs/heads/foo\n",
				"main\n",
				"\n",
				"oldcommit\n",
				"oldcommit\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/foo",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch https://gitlab.com/FORK_OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/foo",
				"git rev-parse foo",
				"git rev-parse refs/remotes/mr-checkout-temp/foo",
				"git fetch https://gitlab.com/FORK_OWNER/REPO.git +refs/heads/feat-new-mr:foo",
				"git update-ref -d refs/remotes/mr-checkout-temp/foo",
				"git config branch.foo.remote https://gitlab.com/FORK_OWNER/REPO.git",
				"git config branch.foo.pushRemote https://gitlab.com/FORK_OWNER/REPO.git",
				"git config branch.foo.merge refs/heads/feat-new-mr",
				"git checkout foo",
			},
			expectedOutput: "✓ Branch 'foo' is up to date with the merge request.\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHTTP := httpmock.New()
			defer fakeHTTP.Verify(t)

			for _, mock := range tc.httpMocks {
				fakeHTTP.RegisterResponder(mock.method, mock.path, httpmock.NewStringResponse(mock.status, mock.body))
			}

			cs, csTeardown := test.InitCmdStubber()
			defer csTeardown()
			for _, stub := range tc.shelloutStubs {
				cs.Stub(stub)
			}

			output, err := runCommand(fakeHTTP, tc.branch, false, tc.commandArgs)

			if assert.NoErrorf(t, err, "error running command `mr checkout %s`: %v", tc.commandArgs, err) {
				if tc.expectedOutput != "" {
					assert.Equal(t, tc.expectedOutput, output.String())
				} else {
					assert.Empty(t, output.String())
				}
				assert.Empty(t, output.Stderr())
			}

			assert.Equal(t, len(tc.expectedShellouts), cs.Count)
			for idx, expectedShellout := range tc.expectedShellouts {
				if idx < len(cs.Calls) {
					assert.Equal(t, expectedShellout, strings.Join(cs.Calls[idx].Args, " "))
				}
			}
		})
	}
}

func TestMrCheckoutWithBranchComparison(t *testing.T) {
	type httpMock struct {
		method string
		path   string
		status int
		body   string
	}

	tests := []struct {
		name              string
		commandArgs       string
		branch            string
		httpMocks         []httpMock
		shelloutStubs     []string
		expectedShellouts []string
		expectedError     bool
		expectedOutput    string
	}{
		{
			name:        "when branch exists and is behind",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"deadbeef\n",
				"newbeef\n",
				"2\n",
				"0\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count feat-new-mr..refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count refs/remotes/mr-checkout-temp/feat-new-mr..feat-new-mr",
				"git status --porcelain",
				"git fetch git@gitlab.com:OWNER/REPO.git +refs/heads/feat-new-mr:feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
				"git config branch.feat-new-mr.remote git@gitlab.com:OWNER/REPO.git",
				"git config branch.feat-new-mr.merge refs/heads/feat-new-mr",
				"git checkout feat-new-mr",
			},
			expectedOutput: "\n⚠️  Your local branch 'feat-new-mr' is 2 commit(s) behind the merge request.\n\nUpdating branch to match the merge request...\n",
		},
		{
			name:        "when branch has local commits",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"deadbeef\n",
				"newbeef\n",
				"0\n",
				"2\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count feat-new-mr..refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count refs/remotes/mr-checkout-temp/feat-new-mr..feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
			},
			expectedError:  true,
			expectedOutput: "\n⚠️  Your local branch 'feat-new-mr' is 2 commit(s) ahead of the merge request.\n\n❌ Cannot update branch: You have local commits that would be lost.\n",
		},
		{
			name:        "when branch has diverged",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"deadbeef\n",
				"newbeef\n",
				"3\n",
				"2\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count feat-new-mr..refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count refs/remotes/mr-checkout-temp/feat-new-mr..feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
			},
			expectedError:  true,
			expectedOutput: "\n⚠️  Your local branch 'feat-new-mr' has diverged from the merge request:\n   - 2 commit(s) ahead (local commits not in MR)\n   - 3 commit(s) behind (MR commits not in local)\n\n❌ Cannot update branch: You have local commits that would be lost.\n",
		},
		{
			name:        "when branch has uncommitted changes",
			commandArgs: "123",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"deadbeef\n",
				"newbeef\n",
				"2\n",
				"0\n",
				"M  file.txt\n?? new-file.txt\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count feat-new-mr..refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-list --count refs/remotes/mr-checkout-temp/feat-new-mr..feat-new-mr",
				"git status --porcelain",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
			},
			expectedError:  true,
			expectedOutput: "\n⚠️  Your local branch 'feat-new-mr' is 2 commit(s) behind the merge request.\n\n❌ Cannot update branch: You have 2 uncommitted change(s) that would be lost.\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHTTP := httpmock.New()
			defer fakeHTTP.Verify(t)

			for _, mock := range tc.httpMocks {
				fakeHTTP.RegisterResponder(mock.method, mock.path, httpmock.NewStringResponse(mock.status, mock.body))
			}

			cs, csTeardown := test.InitCmdStubber()
			defer csTeardown()
			for _, stub := range tc.shelloutStubs {
				cs.Stub(stub)
			}

			output, err := runCommand(fakeHTTP, tc.branch, false, tc.commandArgs)

			if tc.expectedError {
				assert.Error(t, err, "expected error for test case: %s", tc.name)
			} else {
				assert.NoErrorf(t, err, "error running command `mr checkout %s`: %v", tc.commandArgs, err)
				assert.Empty(t, output.Stderr())
			}

			if tc.expectedOutput != "" {
				assert.Equal(t, tc.expectedOutput, output.String())
			}

			assert.Equal(t, len(tc.expectedShellouts), cs.Count, "number of shell commands for test case: %s", tc.name)
			for idx, expectedShellout := range tc.expectedShellouts {
				if idx < len(cs.Calls) {
					assert.Equal(t, expectedShellout, strings.Join(cs.Calls[idx].Args, " "))
				}
			}
		})
	}
}

func TestMatchingProtocolURL(t *testing.T) {
	project := &gitlab.Project{
		SSHURLToRepo:  "git@gitlab.com:owner/repo.git",
		HTTPURLToRepo: "https://gitlab.com/owner/repo.git",
	}

	tests := []struct {
		name        string
		existingURL string
		expected    string
	}{
		{
			name:        "SSH URL returns SSH",
			existingURL: "git@gitlab.com:other/repo.git",
			expected:    "git@gitlab.com:owner/repo.git",
		},
		{
			name:        "SSH protocol URL returns SSH",
			existingURL: "ssh://git@gitlab.com/other/repo.git",
			expected:    "git@gitlab.com:owner/repo.git",
		},
		{
			name:        "HTTPS URL returns HTTPS",
			existingURL: "https://gitlab.com/other/repo.git",
			expected:    "https://gitlab.com/owner/repo.git",
		},
		{
			name:        "HTTP URL returns HTTPS",
			existingURL: "http://gitlab.com/other/repo.git",
			expected:    "https://gitlab.com/owner/repo.git",
		},
		{
			name:        "Unknown protocol returns HTTPS",
			existingURL: "ftp://gitlab.com/other/repo.git",
			expected:    "https://gitlab.com/owner/repo.git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchingProtocolURL(tc.existingURL, project)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMrCheckoutWithSetUpstream(t *testing.T) {
	tests := []struct {
		name              string
		commandArgs       string
		branch            string
		httpMocks         []httpMock
		shelloutStubs     []string
		expectedShellouts []string
		expectedError     bool
		expectedOutput    string
	}{
		{
			name:        "when --set-upstream-to is used with valid remote",
			commandArgs: "123 --set-upstream-to upstream/main",
			branch:      "main",
			httpMocks: []httpMock{
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/OWNER/REPO/merge_requests/123",
					status: http.StatusOK,
					body: `{
						"id": 123,
						"iid": 123,
						"project_id": 3,
						"source_project_id": 3,
						"title": "test mr title",
						"description": "test mr description",
						"allow_collaboration": false,
						"state": "opened",
						"source_branch":"feat-new-mr"
					}`,
				},
				{
					method: http.MethodGet,
					path:   "/api/v4/projects/3",
					status: http.StatusOK,
					body: `{
						"id": 3,
						"ssh_url_to_repo": "git@gitlab.com:OWNER/REPO.git",
						"http_url_to_repo": "https://gitlab.com/OWNER/REPO.git"
					}`,
				},
			},
			shelloutStubs: []string{
				"main\n",
				"branch.main.remote origin\nbranch.main.merge refs/heads/main\n",
				"git@gitlab.com:OWNER/REPO.git\n",
				"refs/heads/feat-new-mr\n",
				"main\n",
				"\n",
				"oldcommit\n",
				"oldcommit\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
				"\n",
			},
			expectedShellouts: []string{
				"git symbolic-ref --quiet --short HEAD",
				"git config --get-regexp ^branch\\.main\\.(remote|merge)$",
				"git config remote.origin.url",
				"git rev-parse --verify refs/heads/feat-new-mr",
				"git symbolic-ref --quiet --short HEAD",
				"git fetch git@gitlab.com:OWNER/REPO.git refs/heads/feat-new-mr:refs/remotes/mr-checkout-temp/feat-new-mr",
				"git rev-parse feat-new-mr",
				"git rev-parse refs/remotes/mr-checkout-temp/feat-new-mr",
				"git fetch git@gitlab.com:OWNER/REPO.git +refs/heads/feat-new-mr:feat-new-mr",
				"git update-ref -d refs/remotes/mr-checkout-temp/feat-new-mr",
				"git config branch.feat-new-mr.remote git@gitlab.com:OWNER/REPO.git",
				"git config branch.feat-new-mr.merge refs/heads/feat-new-mr",
				"git checkout feat-new-mr",
				"git branch --set-upstream-to upstream/main",
			},
			expectedOutput: "✓ Branch 'feat-new-mr' is up to date with the merge request.\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeHTTP := httpmock.New()
			defer fakeHTTP.Verify(t)

			for _, mock := range tc.httpMocks {
				fakeHTTP.RegisterResponder(mock.method, mock.path, httpmock.NewStringResponse(mock.status, mock.body))
			}

			cs, csTeardown := test.InitCmdStubber()
			defer csTeardown()
			for _, stub := range tc.shelloutStubs {
				cs.Stub(stub)
			}

			output, err := runCommand(fakeHTTP, tc.branch, false, tc.commandArgs)

			if tc.expectedError {
				assert.Error(t, err, "expected error for test case: %s", tc.name)
			} else {
				assert.NoErrorf(t, err, "error running command `mr checkout %s`: %v", tc.commandArgs, err)
				assert.Empty(t, output.Stderr())
			}

			if tc.expectedOutput != "" {
				assert.Equal(t, tc.expectedOutput, output.String())
			}

			assert.Equal(t, len(tc.expectedShellouts), cs.Count, "number of shell commands for test case: %s", tc.name)
			for idx, expectedShellout := range tc.expectedShellouts {
				if idx < len(cs.Calls) {
					assert.Equal(t, expectedShellout, strings.Join(cs.Calls[idx].Args, " "))
				}
			}
		})
	}
}

type httpMock struct {
	method string
	path   string
	status int
	body   string
}
