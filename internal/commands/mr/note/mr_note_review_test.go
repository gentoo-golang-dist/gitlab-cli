//go:build !integration

package note

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func setupReviewExec(t *testing.T, testClient *gitlabtesting.TestClient, stdin string) cmdtest.CmdExecFunc {
	t.Helper()
	return cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
		return NewCmdNote(f)
	}, false,
		cmdtest.WithGitLabClient(testClient.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithConfig(config.NewFromString("editor: vi")),
		cmdtest.WithStdin(stdin),
	)
}

func Test_review(t *testing.T) { //nolint:tparallel // subtests mutate package-level vars (GetLatestDiffVersion, ResolveDiscussionID)

	t.Run("general comment", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "LGTM", *opts.Note)
				assert.Nil(t, opts.Position)
				assert.Nil(t, opts.InReplyToDiscussionID)
				return &gitlab.DraftNote{ID: 100}, nil, nil
			})

		input, _ := json.Marshal([]reviewEntry{{Body: "LGTM"}})
		exec := setupReviewExec(t, testClient, string(input))
		output, err := exec("review 1")
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Created draft note 100 (1/1)")
	})

	t.Run("multiple entries", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		var callCount int
		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			Times(3).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				callCount++
				return &gitlab.DraftNote{ID: int64(callCount)}, nil, nil
			})

		entries := []reviewEntry{
			{Body: "First"},
			{Body: "Second"},
			{Body: "Third"},
		}
		input, _ := json.Marshal(entries)
		exec := setupReviewExec(t, testClient, string(input))
		output, err := exec("review 1")
		require.NoError(t, err)
		assert.Contains(t, output.String(), "(1/3)")
		assert.Contains(t, output.String(), "(2/3)")
		assert.Contains(t, output.String(), "(3/3)")
	})

	t.Run("with publish", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.DraftNote{ID: 10}, nil, nil)

		testClient.MockDraftNotes.EXPECT().
			PublishAllDraftNotes("OWNER/REPO", int64(1)).
			Return(nil, nil)

		input, _ := json.Marshal([]reviewEntry{{Body: "Ship it"}})
		exec := setupReviewExec(t, testClient, string(input))
		output, err := exec("review 1 --publish")
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Published 1 draft notes")
	})

	t.Run("diff comment with file and line", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		origGetVersion := mrutils.GetLatestDiffVersion
		mrutils.GetLatestDiffVersion = func(_ *gitlab.Client, _ string, _ int64) (*gitlab.MergeRequestDiffVersion, error) {
			return &gitlab.MergeRequestDiffVersion{
				BaseCommitSHA:  "base",
				HeadCommitSHA:  "head",
				StartCommitSHA: "start",
				Diffs: []*gitlab.Diff{
					{
						NewPath: "main.go",
						OldPath: "main.go",
						Diff:    "@@ -1,3 +1,4 @@\n context\n+added line\n context2\n context3\n",
					},
				},
			}, nil
		}
		t.Cleanup(func() { mrutils.GetLatestDiffVersion = origGetVersion })

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "Fix this", *opts.Note)
				assert.NotNil(t, opts.Position)
				assert.Equal(t, "main.go", *opts.Position.NewPath)
				return &gitlab.DraftNote{ID: 20}, nil, nil
			})

		entries := []reviewEntry{{Body: "Fix this", File: "main.go", Line: "2"}}
		input, _ := json.Marshal(entries)
		exec := setupReviewExec(t, testClient, string(input))
		output, err := exec("review 1")
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Created draft note 20")
	})

	t.Run("reply to discussion", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		origResolve := mrutils.ResolveDiscussionID
		mrutils.ResolveDiscussionID = func(_ *gitlab.Client, _ string, _ int64, prefix string) (string, error) {
			return "abcdef1234567890abcdef1234567890abcdef12", nil
		}
		t.Cleanup(func() { mrutils.ResolveDiscussionID = origResolve })

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(_ any, _ int64, opts *gitlab.CreateDraftNoteOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.DraftNote, *gitlab.Response, error) {
				assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", *opts.InReplyToDiscussionID)
				assert.True(t, *opts.ResolveDiscussion)
				return &gitlab.DraftNote{ID: 30}, nil, nil
			})

		entries := []reviewEntry{{Body: "Agreed", Reply: "abcdef12", Resolve: true}}
		input, _ := json.Marshal(entries)
		exec := setupReviewExec(t, testClient, string(input))
		output, err := exec("review 1")
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Created draft note 30")
	})

	t.Run("empty body errors", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		entries := []reviewEntry{{Body: ""}}
		input, _ := json.Marshal(entries)
		exec := setupReviewExec(t, testClient, string(input))
		_, err := exec("review 1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry 0: empty body")
	})

	t.Run("empty array errors", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := setupReviewExec(t, testClient, "[]")
		_, err := exec("review 1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty review: no comments provided")
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		exec := setupReviewExec(t, testClient, "not json")
		_, err := exec("review 1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON input")
	})

	t.Run("publish API error", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		testClient.MockDraftNotes.EXPECT().
			CreateDraftNote("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.DraftNote{ID: 10}, nil, nil)

		testClient.MockDraftNotes.EXPECT().
			PublishAllDraftNotes("OWNER/REPO", int64(1)).
			Return(nil, assert.AnError)

		input, _ := json.Marshal([]reviewEntry{{Body: "test"}})
		exec := setupReviewExec(t, testClient, string(input))
		_, err := exec("review 1 --publish")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish draft notes")
	})

	t.Run("file not in diff errors", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)
		makeMRMock(t, testClient)

		origGetVersion := mrutils.GetLatestDiffVersion
		mrutils.GetLatestDiffVersion = func(_ *gitlab.Client, _ string, _ int64) (*gitlab.MergeRequestDiffVersion, error) {
			return &gitlab.MergeRequestDiffVersion{
				Diffs: []*gitlab.Diff{
					{NewPath: "other.go", OldPath: "other.go", Diff: "@@ -1 +1 @@\n-old\n+new\n"},
				},
			}, nil
		}
		t.Cleanup(func() { mrutils.GetLatestDiffVersion = origGetVersion })

		entries := []reviewEntry{{Body: "Comment", File: "missing.go", Line: "1"}}
		input, _ := json.Marshal(entries)
		exec := setupReviewExec(t, testClient, string(input))
		_, err := exec("review 1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry 0")
		assert.Contains(t, err.Error(), "not found in MR diff")
	})
}
