//go:build !integration

package checkout

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/test"
)

func setupTest(t *testing.T, testClient *gitlabtesting.TestClient, opts ...cmdtest.FactoryOption) func(string) (*test.CmdOut, error) {
	t.Helper()

	pu, _ := url.Parse("https://gitlab.com/OWNER/REPO.git")

	defaultOpts := []cmdtest.FactoryOption{
		cmdtest.WithGitLabClient(testClient.Client),
		cmdtest.WithBranch("main"),
		func(f *cmdtest.Factory) {
			f.RemotesStub = func() (glrepo.Remotes, error) {
				return glrepo.Remotes{
					{
						Remote: &git.Remote{
							Name:     "upstream",
							Resolved: "base",
							PushURL:  pu,
						},
						Repo: glrepo.New("OWNER", "REPO", glinstance.DefaultHostname),
					},
					{
						Remote: &git.Remote{
							Name:     "origin",
							Resolved: "base",
							PushURL:  pu,
						},
						Repo: glrepo.New("monalisa", "REPO", glinstance.DefaultHostname),
					},
				}, nil
			}
		},
	}

	return cmdtest.SetupCmdForTest(t, NewCmdCheckout, false, append(defaultOpts, opts...)...)
}

func TestMrCheckout(t *testing.T) {
	t.Run("when a valid MR is checked out using MR id", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:                 123,
					IID:                123,
					ProjectID:          3,
					SourceProjectID:    3,
					SourceBranch:       "feat-new-mr",
					Title:              "test mr title",
					Description:        "test mr description",
					AllowCollaboration: false,
					State:              "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
			DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:feat-new-mr"))
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/heads/feat-new-mr").Return("", nil)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
			DoAndReturn(git.CheckoutStub("feat-new-mr"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.NoError(t, err)
		assert.Contains(t, output.Stderr(), "Counting objects")
		assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:feat-new-mr")
		assert.Contains(t, output.Stderr(), "Switched to a new branch 'feat-new-mr'")
	})

	t.Run("when a valid MR comes from a forked private project", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:                 123,
					IID:                123,
					ProjectID:          3,
					SourceProjectID:    3,
					TargetProjectID:    4,
					SourceBranch:       "feat-new-mr",
					Title:              "test mr title",
					Description:        "test mr description",
					AllowCollaboration: false,
					State:              "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(nil, nil, &gitlab.ErrorResponse{Message: "404 Project Not Found"})

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           4,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/merge-requests/123/head:feat-new-mr").
			DoAndReturn(git.FetchStub("refs/merge-requests/123/head:feat-new-mr"))
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/merge-requests/123/head").Return("", nil)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
			DoAndReturn(git.CheckoutStub("feat-new-mr"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.NoError(t, err)
		assert.Contains(t, output.Stderr(), "[new branch] refs/merge-requests/123/head:feat-new-mr")
		assert.Contains(t, output.Stderr(), "Switched to a new branch 'feat-new-mr'")
	})

	t.Run("when a valid MR is checked out using MR id and specifying branch", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:                 123,
					IID:                123,
					ProjectID:          3,
					SourceProjectID:    4,
					SourceBranch:       "feat-new-mr",
					Title:              "test mr title",
					Description:        "test mr description",
					AllowCollaboration: true,
					State:              "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:FORK_OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:FORK_OWNER/REPO.git", "refs/heads/feat-new-mr:foo").
			DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:foo"))
		mockGit.EXPECT().Git("config", "branch.foo.remote", "git@gitlab.com:FORK_OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.foo.pushRemote", "git@gitlab.com:FORK_OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.foo.merge", "refs/heads/feat-new-mr").Return("", nil)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "foo").
			DoAndReturn(git.CheckoutStub("foo"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123 --branch foo")

		assert.NoError(t, err)
		assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:foo")
		assert.Contains(t, output.Stderr(), "Switched to a new branch 'foo'")
	})

	t.Run("when initial fetch fails but retry succeeds", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:              123,
					IID:             123,
					ProjectID:       3,
					SourceProjectID: 3,
					SourceBranch:    "feat-new-mr",
					State:           "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
			DoAndReturn(git.FailingFetchStub("refs/heads/feat-new-mr:feat-new-mr", "couldn't find remote ref"))
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr").
			DoAndReturn(git.FetchStub("refs/heads/feat-new-mr"))
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/heads/feat-new-mr").Return("", nil)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
			DoAndReturn(git.CheckoutStub("feat-new-mr"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.NoError(t, err)
		assert.Contains(t, output.Stderr(), "fetch attempt refs/heads/feat-new-mr:feat-new-mr failed")
		assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr")
		assert.Contains(t, output.Stderr(), "Switched to a new branch 'feat-new-mr'")
	})

	t.Run("when fetch fails completely", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:              123,
					IID:             123,
					ProjectID:       3,
					SourceProjectID: 3,
					SourceBranch:    "feat-new-mr",
					State:           "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
			DoAndReturn(git.FailingFetchStub("refs/heads/feat-new-mr:feat-new-mr", "fetch failed"))
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr").
			DoAndReturn(git.FailingFetchStub("refs/heads/feat-new-mr", "fetch failed"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fetch failed")
		assert.Contains(t, output.Stderr(), "fetch attempt refs/heads/feat-new-mr:feat-new-mr failed")
		assert.Contains(t, output.Stderr(), "fetch attempt refs/heads/feat-new-mr failed")
		assert.Empty(t, output.String())
	})

	t.Run("when checkout fails", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:              123,
					IID:             123,
					ProjectID:       3,
					SourceProjectID: 3,
					SourceBranch:    "feat-new-mr",
					State:           "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
			DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:feat-new-mr"))
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").Return("", nil)
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/heads/feat-new-mr").Return("", nil)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
			DoAndReturn(git.FailingCheckoutStub("feat-new-mr", "pathspec 'feat-new-mr' did not match"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not checkout branch")
		assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:feat-new-mr")
		assert.Contains(t, output.Stderr(), "error: pathspec 'feat-new-mr' did not match")
		assert.Empty(t, output.String())
	})

	t.Run("when git config fails", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:              123,
					IID:             123,
					ProjectID:       3,
					SourceProjectID: 3,
					SourceBranch:    "feat-new-mr",
					State:           "opened",
				},
			}, nil, nil)

		testClient.MockProjects.EXPECT().
			GetProject(gomock.Any(), gomock.Any()).
			Return(&gitlab.Project{
				ID:           3,
				SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
			}, nil, nil)

		ctrl := gomock.NewController(t)
		mockGit := git_testing.NewMockGitRunner(ctrl)
		mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
			DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:feat-new-mr"))
		mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").
			Return("", errors.New("could not set config"))

		exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
		output, err := exec("123")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not set config")
		assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:feat-new-mr")
		assert.Empty(t, output.String())
	})
}

func TestMrCheckout_SetUpstreamTo(t *testing.T) {
	t.Parallel()
	testClient := gitlabtesting.NewTestClient(t)

	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:              123,
				IID:             123,
				ProjectID:       3,
				SourceProjectID: 3,
				SourceBranch:    "feat-new-mr",
				State:           "opened",
			},
		}, nil, nil)

	testClient.MockProjects.EXPECT().
		GetProject(gomock.Any(), gomock.Any()).
		Return(&gitlab.Project{
			ID:           3,
			SSHURLToRepo: "git@gitlab.com:OWNER/REPO.git",
		}, nil, nil)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "git@gitlab.com:OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
		DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:feat-new-mr"))
	mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "git@gitlab.com:OWNER/REPO.git").Return("", nil)
	mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/heads/feat-new-mr").Return("", nil)
	mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
		DoAndReturn(git.CheckoutStub("feat-new-mr"))
	mockGit.EXPECT().Git("branch", "--set-upstream-to", "upstream/main").Return("", nil)

	exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit))
	output, err := exec("123 --set-upstream-to upstream/main")

	assert.NoError(t, err)
	assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:feat-new-mr")
	assert.Contains(t, output.Stderr(), "Switched to a new branch 'feat-new-mr'")
}

func TestMrCheckout_HTTPSProtocolConfiguration(t *testing.T) {
	testClient := gitlabtesting.NewTestClient(t)

	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:                 123,
				IID:                123,
				ProjectID:          3,
				SourceProjectID:    3,
				SourceBranch:       "feat-new-mr",
				Title:              "test mr title",
				Description:        "test mr description",
				AllowCollaboration: false,
				State:              "opened",
			},
		}, nil, nil)

	testClient.MockProjects.EXPECT().
		GetProject(gomock.Any(), gomock.Any()).
		Return(&gitlab.Project{
			ID:            3,
			HTTPURLToRepo: "https://gitlab.com/OWNER/REPO.git",
			SSHURLToRepo:  "git@gitlab.com:OWNER/REPO.git",
		}, nil, nil)

	ctrl := gomock.NewController(t)
	mockGit := git_testing.NewMockGitRunner(ctrl)
	mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "fetch", "https://gitlab.com/OWNER/REPO.git", "refs/heads/feat-new-mr:feat-new-mr").
		DoAndReturn(git.FetchStub("refs/heads/feat-new-mr:feat-new-mr"))
	mockGit.EXPECT().Git("config", "branch.feat-new-mr.remote", "https://gitlab.com/OWNER/REPO.git").Return("", nil)
	mockGit.EXPECT().Git("config", "branch.feat-new-mr.merge", "refs/heads/feat-new-mr").Return("", nil)
	mockGit.EXPECT().GitWithIO(gomock.Any(), gomock.Any(), "checkout", "feat-new-mr").
		DoAndReturn(git.CheckoutStub("feat-new-mr"))

	cfg := config.NewBlankConfig()
	err := cfg.Set("gitlab.com", "git_protocol", "https")
	assert.NoError(t, err)

	exec := setupTest(t, testClient, cmdtest.WithGitRunner(mockGit), cmdtest.WithConfig(cfg))
	output, err := exec("123")

	assert.NoError(t, err)
	assert.Contains(t, output.Stderr(), "[new branch] refs/heads/feat-new-mr:feat-new-mr")
	assert.Contains(t, output.Stderr(), "Switched to a new branch 'feat-new-mr'")
}
