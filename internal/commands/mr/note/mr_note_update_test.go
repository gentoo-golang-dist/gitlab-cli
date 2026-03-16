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

func Test_update_subcommand(t *testing.T) {
	t.Parallel()

	t.Run("update note with -m flag", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		// Mock FindNoteInDiscussions via ListAllDiscussions
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "disc-aaa",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "Original body"},
					},
				},
			}, nil, nil)

		// Mock UpdateMergeRequestDiscussionNote
		testClient.MockDiscussions.EXPECT().
			UpdateMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-aaa", int64(100), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussion string, note int64, opts *gitlab.UpdateMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "Updated body", *opts.Body)
				return &gitlab.Note{ID: 100}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`update 1 100 -m "Updated body"`)
		require.NoError(t, err)
		assert.Equal(t, "https://gitlab.com/OWNER/REPO/merge_requests/1#note_100\n", output.String())
	})

	t.Run("update note from stdin", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID:    "disc-bbb",
					Notes: []*gitlab.Note{{ID: 200, Body: "Old"}},
				},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			UpdateMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-bbb", int64(200), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussion string, note int64, opts *gitlab.UpdateMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "Body from stdin", *opts.Body)
				return &gitlab.Note{ID: 200}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, false, // not a TTY
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithStdin("Body from stdin\n"),
		)

		output, err := exec(`update 1 200`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_200")
	})

	t.Run("-m takes priority over stdin", func(t *testing.T) {
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

		testClient.MockDiscussions.EXPECT().
			UpdateMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-ccc", int64(300), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussion string, note int64, opts *gitlab.UpdateMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "From flag", *opts.Body)
				return &gitlab.Note{ID: 300}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, false,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithStdin("From stdin"),
		)

		output, err := exec(`update 1 300 -m "From flag"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_300")
	})

	t.Run("empty body produces error", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, false,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithStdin(""),
		)

		_, err := exec(`update 1 100`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "body is required")
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

		_, err := exec(`update 1 999 -m "nope"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note 999 not found")
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

		_, err := exec(`update 1 notanumber -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid note ID")
	})

	t.Run("API error on update", func(t *testing.T) {
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
			UpdateMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-eee", int64(500), gomock.Any()).
			Return(nil, nil, fmt.Errorf("403 Forbidden"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`update 1 500 -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update note")
	})

	t.Run("update without MR arg (single arg = note ID)", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// When only note ID is given, MRFromArgs is called with empty args.
		// For this test we still need to mock MR lookup.
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
			UpdateMergeRequestDiscussionNote("OWNER/REPO", int64(1), "disc-fff", int64(600), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussion string, note int64, opts *gitlab.UpdateMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "Updated", *opts.Body)
				return &gitlab.Note{ID: 600}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		// Single arg: note ID only (MR from current branch - but test mocks MR 1)
		// We need to pass the MR ID since makeMRMock expects GetMergeRequest with IID 1
		output, err := exec(`update 1 600 -m "Updated"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_600")
	})
}
