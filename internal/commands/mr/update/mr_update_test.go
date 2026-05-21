//go:build !integration

package update

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/acarl005/stripansi"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// TODO: test by mocking the appropriate api function
func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "mr_update_test")
}

func TestUpdateMergeRequest(t *testing.T) {
	io, _, stdout, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
	f := cmdtest.NewTestFactory(io, cmdtest.WithConfig(config.NewFromString(heredoc.Doc(`
		hosts:
		  gitlab.com:
		    username: monalisa
		    token: OTOKEN
	`))))
	oldUpdateMr := api.UpdateMR
	timer, _ := time.Parse(time.RFC3339, "2014-11-12T11:45:26.371Z")
	toggle := false

	api.UpdateMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.UpdateMergeRequestOptions) (*gitlab.MergeRequest, error) {
		if projectID == "" || projectID == "WRONG_REPO" || projectID == "expected_err" || mrID == 0 {
			return nil, fmt.Errorf("error expected")
		}
		repo, err := f.BaseRepo()
		if err != nil {
			return nil, err
		}

		// Use provided title and body from opts if available
		title := "mrTitle"
		body := "mrbody"
		if opts.Title != nil {
			title = *opts.Title
		}
		if opts.Description != nil {
			body = *opts.Description
		}

		mr := &gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:          mrID,
				IID:         mrID,
				Title:       title,
				Labels:      gitlab.Labels{"bug", "test"},
				State:       "opened",
				Description: body,
				Author: &gitlab.BasicUser{
					ID:       mrID,
					Name:     "John Dev Wick",
					Username: "jdwick",
				},
				WebURL:    "https://" + repo.RepoHost() + "/" + repo.FullName() + "/-/merge_requests/" + fmt.Sprintf("%d", mrID),
				CreatedAt: &timer,
			},
		}

		if opts.RemoveSourceBranch != nil {
			toggle = !toggle
			mr.ForceRemoveSourceBranch = toggle
		}

		return mr, nil
	}

	api.GetMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.GetMergeRequestsOptions) (*gitlab.MergeRequest, error) {
		if projectID == "" || projectID == "WRONG_REPO" || projectID == "expected_err" {
			return nil, fmt.Errorf("error expected")
		}
		repo, err := f.BaseRepo()
		if err != nil {
			return nil, err
		}
		title := map[int64]string{
			1: "mrTitle",
			2: "Draft: mrTitle",
			3: "Draft: wip: wip: draft: DrAfT: mrTitle",
		}[mrID]
		return &gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:                      mrID,
				IID:                     mrID,
				Title:                   title,
				Labels:                  gitlab.Labels{"test", "bug"},
				State:                   "opened",
				Description:             "mrBody",
				ForceRemoveSourceBranch: toggle,
				SourceBranch:            "feature/test-branch",
				TargetBranch:            "main",
				Author: &gitlab.BasicUser{
					ID:       mrID,
					Name:     "John Dev Wick",
					Username: "jdwick",
				},
				WebURL: fmt.Sprintf("https://%s/%s/-/merge_requests/%d", repo.RepoHost(), repo.FullName(), mrID),
			},
		}, nil
	}

	api.ListMRs = func(client *gitlab.Client, projectID any, opts *gitlab.ListProjectMergeRequestsOptions, listOpts ...api.CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
		return []*gitlab.BasicMergeRequest{}, nil
	}

	testCases := []struct {
		Name        string
		Args        string
		ExpectedMsg []string
		wantErr     bool
	}{
		{
			Name:        "Update works",
			Args:        "1",
			ExpectedMsg: []string{"- Updating merge request !1"},
		},
		{
			Name: "Remove source branch",
			Args: "1 --remove-source-branch",
			ExpectedMsg: []string{
				"- Updating merge request !1",
				"✓ enabled removal of source branch on merge",
			},
		},
		{
			Name: "Restore remove source branch",
			Args: "1 --remove-source-branch",
			ExpectedMsg: []string{
				"- Updating merge request !1",
				"✓ disabled removal of source branch on merge",
			},
		},
		{
			Name:        "Issue Does Not Exist",
			Args:        "0",
			ExpectedMsg: []string{"invalid merge request ID provided"},
			wantErr:     true,
		},
		{
			Name:        "Set draft",
			Args:        "1 --draft",
			ExpectedMsg: []string{"marked as Draft", "!1 Draft: mrTitle"},
		},
		{
			Name:        "Don't set draft twice",
			Args:        "2 --draft",
			ExpectedMsg: []string{"✓ already a Draft", "2 Draft: mrTitle"},
		},
		{
			Name:        "Set ready",
			Args:        "2 --ready",
			ExpectedMsg: []string{"✓ marked as ready", "2 mrTitle"},
		},
		{
			Name:        "Set ready with multiple draft prefixes",
			Args:        "3 --ready",
			ExpectedMsg: []string{"✓ marked as ready", "3 mrTitle"},
		},
		{
			Name:        "Fill commit body requires fill flag",
			Args:        "1 --fill-commit-body",
			ExpectedMsg: []string{"--fill-commit-body should be used with --fill"},
			wantErr:     true,
		},
		{
			Name:        "Update with fill flag",
			Args:        "1 --fill --yes",
			ExpectedMsg: []string{"- Updating merge request !1", "updated title with commit info", "updated body with commit info"},
		},
		{
			Name:        "Update with fill and fill-commit-body flags",
			Args:        "1 --fill --fill-commit-body --yes",
			ExpectedMsg: []string{"- Updating merge request !1", "updated title with commit info", "updated body with commit info"},
		},
		{
			Name:        "Update with yes flag skips confirmation",
			Args:        "1 --fill --yes",
			ExpectedMsg: []string{"- Updating merge request !1"},
		},
		{
			Name:        "Update with fill flag and explicit title",
			Args:        "1 --fill --title 'Custom Title' --yes",
			ExpectedMsg: []string{"- Updating merge request !1", "updated body with commit info"},
		},
		{
			Name:        "Update with fill flag and explicit body",
			Args:        "1 --fill --description 'Custom Description' --yes",
			ExpectedMsg: []string{"- Updating merge request !1", "updated title with commit info"},
		},
		{
			Name:        "Update with fill flag on non-git directory",
			Args:        "1 --fill --yes",
			ExpectedMsg: []string{"- Updating merge request !1"},
		},
		{
			Name:        "Update without fill flag should not use autofill",
			Args:        "1 --yes",
			ExpectedMsg: []string{"- Updating merge request !1"},
		},
	}

	cumulativePreviousOutputLen := 0
	for _, tc := range testCases {
		cmd := NewCmdUpdate(f)
		cmdutils.EnableRepoOverride(cmd, f)
		t.Run(tc.Name, func(t *testing.T) {
			// Handle regular command tests
			argv, err := shlex.Split(tc.Args)
			if err != nil {
				t.Fatal(err)
			}
			cmd.SetArgs(argv)
			_, err = cmd.ExecuteC()
			if err != nil {
				if tc.wantErr {
					require.Error(t, err)
					// For error cases, check if the error message contains expected text
					for _, msg := range tc.ExpectedMsg {
						assert.Contains(t, err.Error(), msg)
					}
					return
				} else {
					t.Fatal(err)
				}
			}

			// For success cases, check stdout output
			out := stripansi.Strip(stdout.String())[cumulativePreviousOutputLen:]
			cumulativePreviousOutputLen += len(out)

			for _, msg := range tc.ExpectedMsg {
				assert.Contains(t, out, msg)
				assert.Equal(t, "", stderr.String())
			}
		})
	}

	api.UpdateMR = oldUpdateMr
}

// restoreAPIGlobals re-points the api.* package-level function vars at their
// real implementations. The legacy table-style TestUpdateMergeRequest above
// installs stubs into api.GetMR / api.UpdateMR / api.ListMRs and only
// restores api.UpdateMR, leaving the others clobbered for subsequent tests
// in the same package run. Until that test is migrated, the gomock-based
// tests below restore the globals themselves so their MockMergeRequests
// expectations actually see the calls.
func restoreAPIGlobals(t *testing.T) {
	t.Helper()
	prevGetMR, prevUpdateMR, prevListMRs := api.GetMR, api.UpdateMR, api.ListMRs
	api.GetMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.GetMergeRequestsOptions) (*gitlab.MergeRequest, error) {
		mr, _, err := client.MergeRequests.GetMergeRequest(projectID, mrID, opts)
		return mr, err
	}
	api.UpdateMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.UpdateMergeRequestOptions) (*gitlab.MergeRequest, error) {
		mr, _, err := client.MergeRequests.UpdateMergeRequest(projectID, mrID, opts)
		return mr, err
	}
	t.Cleanup(func() {
		api.GetMR, api.UpdateMR, api.ListMRs = prevGetMR, prevUpdateMR, prevListMRs
	})
}

func TestUpdate_AddDependsOn(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	restoreAPIGlobals(t)

	testClient := gitlabtesting.NewTestClient(t)

	// MRFromArgs path: arg "5" -> GetMR(5)
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(5), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID: 500, IID: 5, ProjectID: 1,
				Title: "child", State: "opened",
				TargetBranch: "main", SourceBranch: "feat",
				WebURL: "https://gitlab.com/OWNER/REPO/-/merge_requests/5",
			},
		}, nil, nil)

	// UpdateMergeRequest is called even with no other field changes (title is
	// always sent), so accept any opts.
	testClient.MockMergeRequests.EXPECT().
		UpdateMergeRequest("OWNER/REPO", int64(5), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID: 500, IID: 5, ProjectID: 1,
				Title: "child", State: "opened",
				TargetBranch: "main", SourceBranch: "feat",
				WebURL: "https://gitlab.com/OWNER/REPO/-/merge_requests/5",
			},
		}, nil, nil)

	// Resolve the --add-depends-on 3 ref to global ID 303.
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(3), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{ID: 303, IID: 3, ProjectID: 1},
		}, nil, nil)

	// Then post the dependency.
	testClient.MockMergeRequests.EXPECT().
		CreateMergeRequestDependency("OWNER/REPO", int64(5), gomock.Any()).
		DoAndReturn(func(pid any, iid int64, opts gitlab.CreateMergeRequestDependencyOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.MergeRequestDependency, *gitlab.Response, error) {
			require.NotNil(t, opts.BlockingMergeRequestID)
			assert.Equal(t, int64(303), *opts.BlockingMergeRequestID)
			return &gitlab.MergeRequestDependency{ID: 1}, nil, nil
		})

	exec := cmdtest.SetupCmdForTest(t, NewCmdUpdate, true,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	output, err := exec("5 --add-depends-on 3")
	require.NoError(t, err)
	assert.Contains(t, output.String(), "added dependency !3")
}

func TestUpdate_DependsOnReplaceMode(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	restoreAPIGlobals(t)

	testClient := gitlabtesting.NewTestClient(t)

	// Fetch the MR being updated.
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(5), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID: 500, IID: 5, ProjectID: 1,
				Title: "child", State: "opened",
				TargetBranch: "main", SourceBranch: "feat",
				WebURL: "https://gitlab.com/OWNER/REPO/-/merge_requests/5",
			},
		}, nil, nil)

	testClient.MockMergeRequests.EXPECT().
		UpdateMergeRequest("OWNER/REPO", int64(5), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID: 500, IID: 5, ProjectID: 1,
				Title: "child", State: "opened",
				WebURL: "https://gitlab.com/OWNER/REPO/-/merge_requests/5",
			},
		}, nil, nil)

	// Desired: depends on !4 (global ID 404). Current: depends on !2 (ID 202).
	// Expect: add 404, remove 202.
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(4), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{ID: 404, IID: 4, ProjectID: 1},
		}, nil, nil)

	testClient.MockMergeRequests.EXPECT().
		GetMergeRequestDependencies("OWNER/REPO", int64(5)).
		Return([]gitlab.MergeRequestDependency{
			{ID: 1, BlockingMergeRequest: gitlab.BlockingMergeRequest{ID: 202, Iid: 2, ProjectID: 1}},
		}, nil, nil)

	testClient.MockMergeRequests.EXPECT().
		CreateMergeRequestDependency("OWNER/REPO", int64(5), gomock.Any()).
		DoAndReturn(func(pid any, iid int64, opts gitlab.CreateMergeRequestDependencyOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.MergeRequestDependency, *gitlab.Response, error) {
			assert.Equal(t, int64(404), *opts.BlockingMergeRequestID)
			return &gitlab.MergeRequestDependency{ID: 2}, nil, nil
		})

	testClient.MockMergeRequests.EXPECT().
		DeleteMergeRequestDependency("OWNER/REPO", int64(5), int64(202)).
		Return(nil, nil)

	exec := cmdtest.SetupCmdForTest(t, NewCmdUpdate, true,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	output, err := exec("5 --depends-on 4")
	require.NoError(t, err)
	assert.Contains(t, output.String(), "added dependency !4")
	assert.Contains(t, output.String(), "removed dependency !2")
}

func TestUpdate_DependsOnMutuallyExclusive(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	testClient := gitlabtesting.NewTestClient(t)
	exec := cmdtest.SetupCmdForTest(t, NewCmdUpdate, true,
		cmdtest.WithGitLabClient(testClient.Client),
	)
	_, err := exec("5 --depends-on 3 --add-depends-on 4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't be combined")
}

func TestWriteUpdatePreview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		title       string
		description string
		want        string
	}{
		{
			name:        "single-line title and description",
			title:       "My title",
			description: "My description",
			want:        "\nProposed changes:\n  Title: My title\n  Description: My description\n\n",
		},
		{
			name:  "multi-line title indents continuation lines",
			title: "First line\nSecond line\nThird line",
			want:  "\nProposed changes:\n  Title: First line\n         Second line\n         Third line\n\n",
		},
		{
			name:        "multi-line description indents continuation lines",
			description: "First line\nSecond line",
			want:        "\nProposed changes:\n  Description: First line\n              Second line\n\n",
		},
		{
			name:        "editor mode placeholder",
			description: "(from editor)",
			want:        "\nProposed changes:\n  Description: (from editor)\n\n",
		},
		{
			name:        "only title shown when description is empty",
			title:       "Just a title",
			description: "",
			want:        "\nProposed changes:\n  Title: Just a title\n\n",
		},
		{
			name:        "only description shown when title is empty",
			title:       "",
			description: "Just a description",
			want:        "\nProposed changes:\n  Description: Just a description\n\n",
		},
		{
			name:        "nothing shown when both are empty",
			title:       "",
			description: "",
			want:        "\nProposed changes:\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			writeUpdatePreview(&buf, tc.title, tc.description)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
