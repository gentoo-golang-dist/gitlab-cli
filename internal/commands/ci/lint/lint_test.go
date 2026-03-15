//go:build !integration

package lint

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCheckPathArg(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, defaultPath, wantPath, wantErr string
		args                                 []string
	}{
		{name: "defaults to configured path", defaultPath: ".gitlab-ci.yml", wantPath: ".gitlab-ci.yml"},
		{name: "rejects multiple paths", defaultPath: ".gitlab-ci.yml", args: []string{"one", "two"}, wantErr: "too many filepath args"},
		{name: "errors when no path can be derived", wantErr: "path is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := options{defaultPath: tt.defaultPath}
			err := opts.checkPathArg(tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.wantPath, opts.path)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestComplete(t *testing.T) {
	t.Run("uses explicit ref unchanged", func(t *testing.T) {
		opts := newLintOptions(t)
		opts.ref = "user-ref"

		stubGitRefs(t,
			func() (string, error) { t.Fatal("unexpected describeByTags call"); return "", nil },
			func() (string, error) { t.Fatal("unexpected currentBranch call"); return "", nil },
			func(repo glrepo.Interface, client *gitlab.Client) string {
				t.Fatal("unexpected resolveDefaultRef call")
				return ""
			},
		)

		err, tagErr, branchErr := opts.complete([]string{testdataPath(".gitlab-ci.yaml")})
		require.NoError(t, err)
		require.NoError(t, tagErr)
		require.NoError(t, branchErr)
		require.Equal(t, "user-ref", opts.ref)
	})

	t.Run("prefers current branch over reachable tag", func(t *testing.T) {
		opts := newLintOptions(t)

		stubGitRefs(t,
			func() (string, error) { return "v1.1.2-35-gdeadbeef", nil },
			func() (string, error) { return "develop", nil },
			func(repo glrepo.Interface, client *gitlab.Client) string {
				t.Fatal("unexpected resolveDefaultRef call")
				return ""
			},
		)

		err, tagErr, branchErr := opts.complete([]string{testdataPath(".gitlab-ci.yaml")})
		require.NoError(t, err)
		require.NoError(t, tagErr)
		require.NoError(t, branchErr)
		require.Equal(t, "develop", opts.ref)
	})

	t.Run("falls back to default branch and enables dry run for render yaml", func(t *testing.T) {
		opts := newLintOptions(t)
		opts.isRenderYAML = true

		stubGitRefs(t,
			func() (string, error) { return "", errors.New("no tag") },
			func() (string, error) { return "", errors.New("no branch") },
			func(repo glrepo.Interface, client *gitlab.Client) string { return "main" },
		)

		err, tagErr, branchErr := opts.complete([]string{testdataPath(".gitlab-ci.yaml")})
		require.NoError(t, err)
		require.Error(t, tagErr)
		require.Error(t, branchErr)
		require.Equal(t, "main", opts.ref)
		require.True(t, opts.isDryRun)
	})

	t.Run("falls back to tag when branch lookup fails", func(t *testing.T) {
		opts := newLintOptions(t)

		stubGitRefs(t,
			func() (string, error) { return "v1.2.3", nil },
			func() (string, error) { return "", errors.New("no branch") },
			nil,
		)

		err, tagErr, branchErr := opts.complete([]string{testdataPath(".gitlab-ci.yaml")})
		require.NoError(t, err)
		require.NoError(t, tagErr)
		require.NoError(t, branchErr)
		require.Equal(t, "v1.2.3", opts.ref)
	})
}

func TestEnrichErrorIfNoGit(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("api failed")

	for _, tt := range []struct {
		name, userRef, want string
	}{
		{name: "joins errors when git resolution failed without user ref", want: "couldn't resolve ref: no tag or branch found for current repo."},
		{name: "leaves api error unchanged otherwise", userRef: "scratch", want: "api failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := (&options{userRef: tt.userRef}).enrichErrorIfNoGit(apiErr, errors.New("no tag"), errors.New("no branch"))
			require.Error(t, err)
			assert.ErrorIs(t, err, apiErr)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRun(t *testing.T) {
	t.Run("assembles request for summary mode and logs progress when not spinnable", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		testClient.MockProjects.EXPECT().
			GetProject("OWNER/REPO", gomock.Any()).
			Return(&gitlab.Project{ID: 123}, nil, nil)
		testClient.MockValidate.EXPECT().
			ProjectNamespaceLint(int64(123), gomock.Any()).
			DoAndReturn(func(pid any, opt *gitlab.ProjectNamespaceLintOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectLintResult, *gitlab.Response, error) {
				require.Equal(t, "release", *opt.Ref)
				require.False(t, *opt.DryRun)
				require.True(t, *opt.IncludeJobs)
				require.Contains(t, *opt.Content, "GO_VERSION")
				return &gitlab.ProjectLintResult{Valid: true}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, cmdtest.WithGitLabClient(testClient.Client))
		out, err := exec("--ref release --include-jobs " + testdataPath(".gitlab-ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, "✓ CI/CD YAML is valid!\n", out.OutBuf.String())
		require.Equal(t, "validating...\n", out.ErrBuf.String())
	})

	t.Run("render yaml keeps stdout clean and auto enables dry run", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		testClient.MockProjects.EXPECT().
			GetProject("OWNER/REPO", gomock.Any()).
			Return(&gitlab.Project{ID: 123}, nil, nil)
		testClient.MockValidate.EXPECT().
			ProjectNamespaceLint(int64(123), gomock.Any()).
			DoAndReturn(func(pid any, opt *gitlab.ProjectNamespaceLintOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectLintResult, *gitlab.Response, error) {
				require.Equal(t, "scratch", *opt.Ref)
				require.True(t, *opt.DryRun)
				require.False(t, *opt.IncludeJobs)
				return &gitlab.ProjectLintResult{
					Valid:      true,
					MergedYaml: "merged:\n  yaml: true\n",
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, cmdtest.WithGitLabClient(testClient.Client))
		out, err := exec("--render-yaml --ref scratch " + testdataPath(".gitlab-ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, "merged:\n  yaml: true\n", out.OutBuf.String())
		require.Equal(t, "validating...\n", out.ErrBuf.String())
	})

	t.Run("setup failures remain intact", func(t *testing.T) {
		tests := []struct {
			name        string
			factoryOpts []cmdtest.FactoryOption
			args        string
			wantErr     string
			setupMock   func(tc *gitlabtesting.TestClient)
		}{
			{
				name:    "missing path",
				args:    "WRONG_PATH",
				wantErr: "WRONG_PATH: no such file or directory",
				setupMock: func(tc *gitlabtesting.TestClient) {
					tc.MockProjects.EXPECT().
						GetProject("OWNER/REPO", gomock.Any()).
						Return(&gitlab.Project{ID: 123}, nil, nil)
				},
			},
			{
				name:        "base repo failure",
				args:        testdataPath(".gitlab-ci.yaml"),
				wantErr:     "you must be in a GitLab project repository for this action.",
				factoryOpts: []cmdtest.FactoryOption{cmdtest.WithBaseRepoError(errors.New("no base repo present"))},
			},
			{
				name:        "gitlab client failure",
				args:        testdataPath(".gitlab-ci.yaml"),
				wantErr:     "client failed",
				factoryOpts: []cmdtest.FactoryOption{cmdtest.WithGitLabClientError(errors.New("client failed"))},
			},
			{
				name:    "project lookup failure",
				args:    testdataPath(".gitlab-ci.yaml"),
				wantErr: "you must be in a GitLab project repository for this action.",
				setupMock: func(tc *gitlabtesting.TestClient) {
					tc.MockProjects.EXPECT().
						GetProject("OWNER/REPO", gomock.Any()).
						Return(nil, nil, errors.New("project failed"))
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var opts []cmdtest.FactoryOption
				opts = append(opts, tt.factoryOpts...)

				if tt.setupMock != nil {
					testClient := gitlabtesting.NewTestClient(t)
					tt.setupMock(testClient)
					opts = append(opts, cmdtest.WithGitLabClient(testClient.Client))
				}

				exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, opts...)
				_, err := exec(tt.args)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("api failure is enriched only when implicit ref fallback fails", func(t *testing.T) {
		t.Run("implicit ref", func(t *testing.T) {
			stubGitRefs(t,
				func() (string, error) { return "", errors.New("no tag") },
				func() (string, error) { return "", errors.New("no branch") },
				func(repo glrepo.Interface, client *gitlab.Client) string { return "main" },
			)

			testClient := gitlabtesting.NewTestClient(t)
			testClient.MockProjects.EXPECT().
				GetProject("OWNER/REPO", gomock.Any()).
				Return(&gitlab.Project{ID: 123}, nil, nil)
			testClient.MockValidate.EXPECT().
				ProjectNamespaceLint(int64(123), gomock.Any()).
				Return(nil, nil, errors.New("api failed"))

			exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, cmdtest.WithGitLabClient(testClient.Client))
			_, err := exec(testdataPath(".gitlab-ci.yaml"))
			require.Error(t, err)
			require.Contains(t, err.Error(), "couldn't resolve ref: no tag or branch found for current repo.")
		})

		t.Run("implicit ref prefers branch when both branch and tag exist", func(t *testing.T) {
			stubGitRefs(t,
				func() (string, error) { return "v1.1.2-35-gdeadbeef", nil },
				func() (string, error) { return "develop", nil },
				func(repo glrepo.Interface, client *gitlab.Client) string {
					t.Fatal("unexpected resolveDefaultRef call")
					return ""
				},
			)

			testClient := gitlabtesting.NewTestClient(t)
			testClient.MockProjects.EXPECT().
				GetProject("OWNER/REPO", gomock.Any()).
				Return(&gitlab.Project{ID: 123}, nil, nil)
			testClient.MockValidate.EXPECT().
				ProjectNamespaceLint(int64(123), gomock.Any()).
				DoAndReturn(func(pid any, opt *gitlab.ProjectNamespaceLintOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectLintResult, *gitlab.Response, error) {
					require.Equal(t, "develop", *opt.Ref)
					return &gitlab.ProjectLintResult{Valid: true, MergedYaml: "merged\n"}, nil, nil
				})

			exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, cmdtest.WithGitLabClient(testClient.Client))
			out, err := exec("--render-yaml " + testdataPath(".gitlab-ci.yaml"))
			require.NoError(t, err)
			require.Equal(t, "merged\n", out.OutBuf.String())
		})

		t.Run("explicit ref", func(t *testing.T) {
			testClient := gitlabtesting.NewTestClient(t)
			testClient.MockProjects.EXPECT().
				GetProject("OWNER/REPO", gomock.Any()).
				Return(&gitlab.Project{ID: 123}, nil, nil)
			testClient.MockValidate.EXPECT().
				ProjectNamespaceLint(int64(123), gomock.Any()).
				Return(nil, nil, errors.New("api failed"))

			exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, cmdtest.WithGitLabClient(testClient.Client))
			_, err := exec("--ref scratch " + testdataPath(".gitlab-ci.yaml"))
			require.Error(t, err)
			require.Equal(t, "api failed", err.Error())
		})
	})
}
