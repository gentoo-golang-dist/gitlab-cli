//go:build !integration

package draft

import (
	"encoding/json"
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

func makeMRMock(t *testing.T, testClient *gitlabtesting.TestClient) {
	t.Helper()
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:     1,
				IID:    1,
				WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
			},
		}, nil, nil)
}

func setupExec(t *testing.T, testClient *gitlabtesting.TestClient, isTTY bool, extraOpts ...cmdtest.FactoryOption) cmdtest.CmdExecFunc {
	t.Helper()
	opts := []cmdtest.FactoryOption{
		cmdtest.WithGitLabClient(testClient.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithConfig(config.NewFromString("editor: vi")),
	}
	opts = append(opts, extraOpts...)
	return cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
		return NewCmdDraft(f)
	}, isTTY, opts...)
}

// --- draft create tests ---

func Test_draft_create(t *testing.T) {
	t.Parallel()

	t.Run("create general draft note", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "Hello draft", *opts.Note)
				assert.Nil(t, opts.Position)
				assert.Nil(t, opts.InReplyToDiscussionID)
				return &gitlab.DraftNote{ID: 10}, nil, nil
			})

		exec := setupExec(t, testClient, true)
		output, err := exec(`create 1 -m "Hello draft"`)
		require.NoError(t, err)
		assert.Equal(t, "Created draft note 10 on !1\n", output.String())
	})

	t.Run("create draft note from stdin", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "Stdin body", *opts.Note)
				return &gitlab.DraftNote{ID: 11}, nil, nil
			})

		exec := setupExec(t, testClient, false, cmdtest.WithStdin("Stdin body\n"))
		output, err := exec(`create 1`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Created draft note 11")
	})

	t.Run("create draft reply with resolve", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		// ResolveDiscussionID calls ListAllDiscussions
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{ID: "abcdef1234567890abcdef1234567890abcdef12"},
			}, nil, nil)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", *opts.InReplyToDiscussionID)
				assert.True(t, *opts.ResolveDiscussion)
				return &gitlab.DraftNote{ID: 12}, nil, nil
			})

		exec := setupExec(t, testClient, true)
		output, err := exec(`create 1 --reply abcdef12 --resolve -m "Fixed"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Created draft note 12")
	})

	t.Run("empty message errors", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := setupExec(t, testClient, false, cmdtest.WithStdin(""))
		_, err := exec(`create 1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty message")
	})

	t.Run("file and reply are mutually exclusive", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)

		exec := setupExec(t, testClient, true)
		_, err := exec(`create 1 --file main.go --reply abc12345 -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "if any flags in the group [file reply] are set none of the others can be")
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			Return(nil, nil, fmt.Errorf("500 Internal Server Error"))

		exec := setupExec(t, testClient, true)
		_, err := exec(`create 1 -m "fail"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create draft note")
	})
}

// --- draft list tests ---

func Test_draft_list(t *testing.T) {
	t.Parallel()

	t.Run("list draft notes human output", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			ListDraftNotes("OWNER/REPO", int64(1), gomock.Nil()).
			Return([]*gitlab.DraftNote{
				{ID: 1, Note: "General comment"},
				{ID: 2, Note: "Diff comment", Position: &gitlab.NotePosition{NewPath: "main.go", NewLine: 42}},
				{ID: 3, Note: "Reply", DiscussionID: "abcdef1234567890"},
			}, nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`list 1`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "1  [general]  General comment")
		assert.Contains(t, output.String(), "2  [main.go:42]  Diff comment")
		assert.Contains(t, output.String(), "3  [reply:abcdef12]  Reply")
	})

	t.Run("list draft notes JSON output", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			ListDraftNotes("OWNER/REPO", int64(1), gomock.Nil()).
			Return([]*gitlab.DraftNote{
				{ID: 1, Note: "Hello"},
			}, nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`list 1 --json`)
		require.NoError(t, err)

		var drafts []gitlab.DraftNote
		require.NoError(t, json.Unmarshal(output.OutBuf.Bytes(), &drafts))
		assert.Len(t, drafts, 1)
		assert.Equal(t, "Hello", drafts[0].Note)
	})

	t.Run("list empty draft notes", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			ListDraftNotes("OWNER/REPO", int64(1), gomock.Nil()).
			Return([]*gitlab.DraftNote{}, nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`list 1`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "No draft notes found.")
	})
}

// --- draft update tests ---

func Test_draft_update(t *testing.T) {
	t.Parallel()

	t.Run("update draft note", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			UpdateDraftNote("OWNER/REPO", int64(1), int64(42), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, _ int64, opts *gitlab.UpdateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "New body", *opts.Note)
				return &gitlab.DraftNote{ID: 42}, nil, nil
			})

		exec := setupExec(t, testClient, true)
		output, err := exec(`update 1 42 -m "New body"`)
		require.NoError(t, err)
		assert.Equal(t, "Updated draft note 42 on !1\n", output.String())
	})

	t.Run("update draft note from stdin", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			UpdateDraftNote("OWNER/REPO", int64(1), int64(42), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, _ int64, opts *gitlab.UpdateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "Stdin update", *opts.Note)
				return &gitlab.DraftNote{ID: 42}, nil, nil
			})

		exec := setupExec(t, testClient, false, cmdtest.WithStdin("Stdin update\n"))
		output, err := exec(`update 1 42`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Updated draft note 42")
	})

	t.Run("update with empty body errors", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := setupExec(t, testClient, false, cmdtest.WithStdin(""))
		_, err := exec(`update 1 42`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "body is required")
	})

	t.Run("invalid draft ID", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)

		exec := setupExec(t, testClient, true)
		_, err := exec(`update 1 notanumber -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid draft note ID")
	})
}

// --- draft delete tests ---

func Test_draft_delete(t *testing.T) {
	t.Parallel()

	t.Run("delete draft note with --yes", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			DeleteDraftNote("OWNER/REPO", int64(1), int64(42)).
			Return(nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`delete 1 42 --yes`)
		require.NoError(t, err)
		assert.Equal(t, "✓ Draft note 42 deleted from !1\n", output.String())
	})

	t.Run("delete non-TTY without --yes errors", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := setupExec(t, testClient, false)
		_, err := exec(`delete 1 42`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot prompt for confirmation")
	})

	t.Run("invalid draft ID", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)

		exec := setupExec(t, testClient, true)
		_, err := exec(`delete 1 abc --yes`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid draft note ID")
	})

	t.Run("API error on delete", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			DeleteDraftNote("OWNER/REPO", int64(1), int64(42)).
			Return(nil, fmt.Errorf("404 Not Found"))

		exec := setupExec(t, testClient, true)
		_, err := exec(`delete 1 42 --yes`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete draft note")
	})
}

// --- draft publish tests ---

func Test_draft_publish(t *testing.T) {
	t.Parallel()

	t.Run("publish single draft", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			PublishDraftNote("OWNER/REPO", int64(1), int64(123)).
			Return(nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`publish 1 123`)
		require.NoError(t, err)
		assert.Equal(t, "✓ Published draft note 123 on !1\n", output.String())
	})

	t.Run("publish all drafts", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			PublishAllDraftNotes("OWNER/REPO", int64(1)).
			Return(nil, nil)

		exec := setupExec(t, testClient, true)
		output, err := exec(`publish 1 --all`)
		require.NoError(t, err)
		assert.Equal(t, "✓ Published all draft notes on !1\n", output.String())
	})

	t.Run("publish no args at all errors", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)

		exec := setupExec(t, testClient, true)
		_, err := exec(`publish`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "specify a draft note ID or use --all")
	})

	t.Run("publish API error", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			PublishDraftNote("OWNER/REPO", int64(1), int64(99)).
			Return(nil, fmt.Errorf("404 Not Found"))

		exec := setupExec(t, testClient, true)
		_, err := exec(`publish 1 99`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish draft note")
	})

	t.Run("publish all API error", func(t *testing.T) {
		t.Parallel()
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			PublishAllDraftNotes("OWNER/REPO", int64(1)).
			Return(nil, fmt.Errorf("500 Internal Server Error"))

		exec := setupExec(t, testClient, true)
		_, err := exec(`publish 1 --all`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish all draft notes")
	})
}
