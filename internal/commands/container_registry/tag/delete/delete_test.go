//go:build !integration

package delete

import (
	"fmt"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_TagDelete(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockContainerRegistry.EXPECT().
		GetRegistryRepositoryTagDetail("OWNER/REPO", int64(101), "latest").
		Return(&gitlab.RegistryRepositoryTag{
			Name: "latest",
			Path: "OWNER/REPO/app:latest",
		}, nil, nil)
	testClient.MockContainerRegistry.EXPECT().
		DeleteRegistryRepositoryTag("OWNER/REPO", int64(101), "latest").
		Return(nil, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	out, err := exec("101 latest --yes")
	require.NoError(t, err)
	assert.Equal(t, heredoc.Doc(`• Deleting container registry tag OWNER/REPO/app:latest
		✓ Container registry tag "latest" deleted.
	`), out.String())
	assert.Empty(t, out.Stderr())
}

func Test_TagDelete_RequiresYesWhenNotInteractive(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)

	_, err := exec("101 latest")
	require.Error(t, err)
	assert.Equal(t, "--yes or -y flag is required when not running interactively", err.Error())
}

func Test_TagDelete_APIError(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockContainerRegistry.EXPECT().
		GetRegistryRepositoryTagDetail("OWNER/REPO", int64(101), "latest").
		Return(&gitlab.RegistryRepositoryTag{Name: "latest", Path: "OWNER/REPO/app:latest"}, nil, nil)
	testClient.MockContainerRegistry.EXPECT().
		DeleteRegistryRepositoryTag("OWNER/REPO", int64(101), "latest").
		Return(nil, fmt.Errorf("api failed"))

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	_, err := exec("101 latest --yes")
	require.Error(t, err)
	assert.Equal(t, "api failed", err.Error())

	var exitErr *cmdutils.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, "failed to delete container registry tag.", exitErr.Details)
}

func Test_TagDelete_DetailAPIError(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockContainerRegistry.EXPECT().
		GetRegistryRepositoryTagDetail("OWNER/REPO", int64(101), "latest").
		Return(nil, nil, fmt.Errorf("api failed"))

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(testClient.Client),
	)

	_, err := exec("101 latest --yes")
	require.Error(t, err)
	assert.Equal(t, "api failed", err.Error())

	var exitErr *cmdutils.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, `failed to fetch container registry tag "latest".`, exitErr.Details)
}
