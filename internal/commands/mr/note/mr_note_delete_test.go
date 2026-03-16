//go:build !integration

package note

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_delete_subcommand(t *testing.T) {
	t.Parallel()

	t.Run("delete note with --yes", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		// Mock FindNoteInDiscussions
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-aaa",
					Notes: []*gitlab.Note{{ID: 100, Body: "To be deleted"}},
				},
			}, nil, nil)

		// Mock DeleteMergeRequestDiscussionNote
		testClient.MockDiscussions.EXPECT().
			DeleteMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-aaa", int64(100)).
			Return(nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`delete 1 100 --yes`)
		require.NoError(t, err)
		assert.Equal(t, "✓ Note 100 deleted from !1\n", output.String())
	})

	t.Run("delete note with -y shorthand", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-bbb",
					Notes: []*gitlab.Note{{ID: 200}},
				},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			DeleteMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-bbb", int64(200)).
			Return(nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`delete 1 200 -y`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Note 200 deleted")
	})

	t.Run("no TTY without --yes produces error", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-ccc",
					Notes: []*gitlab.Note{{ID: 300}},
				},
			}, nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, false, // not a TTY
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`delete 1 300`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot prompt for confirmation")
	})

	t.Run("invalid note ID", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`delete 1 notanumber --yes`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid note ID")
	})

	t.Run("note not found", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-ddd",
					Notes: []*gitlab.Note{{ID: 400}},
				},
			}, nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`delete 1 999 --yes`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note 999 not found")
	})

	t.Run("API error on delete", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-eee",
					Notes: []*gitlab.Note{{ID: 500}},
				},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			DeleteMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-eee", int64(500)).
			Return(nil, fmt.Errorf("403 Forbidden"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`delete 1 500 --yes`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete note")
	})

	t.Run("delete without MR arg (single arg = note ID)", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-fff",
					Notes: []*gitlab.Note{{ID: 600}},
				},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			DeleteMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-fff", int64(600)).
			Return(nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`delete 1 600 --yes`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Note 600 deleted")
	})
}
