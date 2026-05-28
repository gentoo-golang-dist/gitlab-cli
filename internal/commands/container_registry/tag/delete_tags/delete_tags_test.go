//go:build !integration

package delete_tags

import (
	"fmt"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_DeleteTags(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockContainerRegistry.EXPECT().
		DeleteRegistryRepositoryTags("OWNER/REPO", int64(101), gomock.Any()).
		DoAndReturn(func(pid any, repository int64, opt *gitlab.DeleteRegistryRepositoryTagsOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
			assert.Equal(t, "^release-.*", *opt.NameRegexpDelete)
			assert.Equal(t, "^latest$", *opt.NameRegexpKeep)
			assert.Equal(t, int64(5), *opt.KeepN)
			assert.Equal(t, "30d", *opt.OlderThan)

			return nil, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	out, err := exec("101 --name-regex-delete '^release-.*' --name-regex-keep '^latest$' --keep-n 5 --older-than 30d --yes")
	require.NoError(t, err)
	assert.Equal(t, heredoc.Doc(`• Scheduling container registry tags for deletion repo=OWNER/REPO repository=101
		✓ Container registry tags scheduled for deletion. They may remain visible until GitLab finishes the background deletion job.
	`), out.String())
	assert.Empty(t, out.Stderr())
}

func Test_DeleteTags_RequiresNameRegexDelete(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)

	_, err := exec("101 --yes")
	require.Error(t, err)
	assert.Equal(t, "--name-regex-delete is required", err.Error())
}

func Test_DeleteTags_RequiresConfirmationWithFilters(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, true)

	out, err := exec("101 --name-regex-delete '^release-.*' --name-regex-keep '^latest$' --keep-n 5 --older-than 30d")
	require.Error(t, err)
	assert.Equal(t, "user cancelled", err.Error())
	assert.Contains(t, out.String(), "Are you ABSOLUTELY SURE you wish to schedule matching container registry tags for deletion from repository 101?")
	assert.Contains(t, out.Stderr(), "This action schedules container registry tags for deletion from repository 101 on OWNER/REPO.")
	assert.Contains(t, out.Stderr(), "name regex delete: ^release-.*")
	assert.Contains(t, out.Stderr(), "name regex keep: ^latest$")
	assert.Contains(t, out.Stderr(), "keep latest: 5")
	assert.Contains(t, out.Stderr(), "older than: 30d")
	assert.Contains(t, out.Stderr(), "The matching tags may remain visible until the background deletion job has completed.")
}

func Test_DeleteTags_RejectsNegativeKeepN(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)

	_, err := exec("101 --name-regex-delete '.*' --keep-n -1 --yes")
	require.Error(t, err)
	assert.Equal(t, "--keep-n must be zero or a positive integer", err.Error())
}

func Test_DeleteTags_APIError(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockContainerRegistry.EXPECT().
		DeleteRegistryRepositoryTags("OWNER/REPO", int64(101), gomock.Any()).
		Return(nil, fmt.Errorf("api failed"))

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	_, err := exec("101 --name-regex-delete '.*' --yes")
	require.Error(t, err)
	assert.Equal(t, "api failed", err.Error())

	var exitErr *cmdutils.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, "failed to delete container registry tags.", exitErr.Details)
}
