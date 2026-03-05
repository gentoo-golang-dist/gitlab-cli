//go:build !integration

package note

import (
	"errors"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/survivorbat/huhtest"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestMain(m *testing.M) {
	cmdtest.InitTest(m, "mr_note_create_test")
}

func Test_NewCmdNote(t *testing.T) {
	t.Parallel()

	t.Run("--message flag specified", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock CreateMergeRequestDiscussion
		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "Here is my note", *opts.Body)
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --message "Here is my note"`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Equal(t, "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n", output.String())
	})

	t.Run("merge request not found", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest - returns 404
		notFoundResp := &gitlab.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		}
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(122), gomock.Any()).
			Return(nil, notFoundResp, gitlab.ErrNotFound)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`122`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Not Found")
	})
}

func Test_NewCmdNote_error(t *testing.T) {
	t.Parallel()

	t.Run("note could not be created", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock CreateMergeRequestDiscussion - returns 401
		unauthorizedResp := &gitlab.Response{
			Response: &http.Response{StatusCode: http.StatusUnauthorized},
		}
		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			Return(nil, unauthorizedResp, errors.New("401 Unauthorized"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 -m "Some message"`)
		require.Error(t, err)
	})
}

func Test_mrNoteCreate_prompt(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	t.Run("message provided", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock CreateMergeRequestDiscussion
		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Contains(t, *opts.Body, "some note message")
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		responder := huhtest.NewResponder()
		responder.AddResponse("Note message:", "some note message")

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithResponder(t, responder),
		)

		output, err := exec(`1`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301")
	})

	t.Run("message is empty", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		responder := huhtest.NewResponder()
		responder.AddResponse("Note message:", "")

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithResponder(t, responder),
		)

		_, err := exec(`1`)
		require.Error(t, err)
		assert.Equal(t, "aborted... Note has an empty message.", err.Error())
	})
}

func Test_mrNoteCreate_no_duplicate(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	t.Run("message provided", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestNotes - returns existing notes including the duplicate
		testClient.MockNotes.EXPECT().
			ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Note{
				{ID: 0, Body: "aaa"},
				{ID: 111, Body: "bbb"},
				{ID: 222, Body: "some note message"},
				{ID: 333, Body: "ccc"},
			}, nil, nil)

		responder := huhtest.NewResponder()
		responder.AddResponse("Note message:", "some note message")

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithResponder(t, responder),
		)

		output, err := exec(`1 --unique`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_222")
	})
}

func Test_mrNoteCreate_no_duplicate_paginated(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	t.Run("duplicate found on second page", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestNotes - page 1 then page 2
		gomock.InOrder(
			testClient.MockNotes.EXPECT().
				ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
				DoAndReturn(func(pid any, mrIID int64, opts *gitlab.ListMergeRequestNotesOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Note, *gitlab.Response, error) {
					return []*gitlab.Note{
							{ID: 100, Body: "first note"},
							{ID: 101, Body: "second note"},
						}, &gitlab.Response{
							Response: &http.Response{StatusCode: http.StatusOK},
							NextPage: 2,
						}, nil
				}),
			testClient.MockNotes.EXPECT().
				ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
				DoAndReturn(func(pid any, mrIID int64, opts *gitlab.ListMergeRequestNotesOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Note, *gitlab.Response, error) {
					assert.Equal(t, int64(2), opts.Page)
					return []*gitlab.Note{
							{ID: 200, Body: "target note"},
						}, &gitlab.Response{
							Response: &http.Response{StatusCode: http.StatusOK},
							NextPage: 0,
						}, nil
				}),
		)

		responder := huhtest.NewResponder()
		responder.AddResponse("Note message:", "target note")

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithResponder(t, responder),
		)

		output, err := exec(`1 --unique`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_200")
	})

	t.Run("no duplicate across all pages creates new note", func(t *testing.T) {
		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestNotes - page 1 then page 2 (no match on either)
		gomock.InOrder(
			testClient.MockNotes.EXPECT().
				ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
				DoAndReturn(func(pid any, mrIID int64, opts *gitlab.ListMergeRequestNotesOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Note, *gitlab.Response, error) {
					return []*gitlab.Note{
							{ID: 100, Body: "first note"},
						}, &gitlab.Response{
							Response: &http.Response{StatusCode: http.StatusOK},
							NextPage: 2,
						}, nil
				}),
			testClient.MockNotes.EXPECT().
				ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
				DoAndReturn(func(pid any, mrIID int64, opts *gitlab.ListMergeRequestNotesOptions, options ...gitlab.RequestOptionFunc) ([]*gitlab.Note, *gitlab.Response, error) {
					return []*gitlab.Note{
							{ID: 200, Body: "other note"},
						}, &gitlab.Response{
							Response: &http.Response{StatusCode: http.StatusOK},
							NextPage: 0,
						}, nil
				}),
		)

		// Mock CreateMergeRequestDiscussion - note is new
		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "brand new note", *opts.Body)
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 -m "brand new note" --unique`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301")
	})
}

func Test_mrNote_resolve(t *testing.T) {
	t.Parallel()

	t.Run("resolve discussion by note ID", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestDiscussions
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "abc123",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "First discussion"},
					},
				},
				{
					ID: "def456",
					Notes: []*gitlab.Note{
						{ID: 200, Body: "Second discussion"},
						{ID: 201, Body: "Reply to second"},
					},
				},
			}, nil, nil)

		// Mock ResolveMergeRequestDiscussion
		resolved := true
		testClient.MockDiscussions.EXPECT().
			ResolveMergeRequestDiscussion("OWNER/REPO", int64(1), "def456", gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussionID string, opts *gitlab.ResolveMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, &resolved, opts.Resolved)
				return &gitlab.Discussion{ID: "def456"}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --resolve 200`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "✓ Discussion resolved (note #200 in !1)")
	})

	t.Run("note not found", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestDiscussions - note 999 doesn't exist
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "abc123",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "First discussion"},
					},
				},
			}, nil, nil)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 --resolve 999`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note 999 not found in MR !1")
	})
}

func Test_mrNote_unresolve(t *testing.T) {
	t.Parallel()

	t.Run("unresolve discussion by note ID", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		// Mock GetMergeRequest
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
			Return(&gitlab.MergeRequest{
				BasicMergeRequest: gitlab.BasicMergeRequest{
					ID:     1,
					IID:    1,
					WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
				},
			}, nil, nil)

		// Mock ListMergeRequestDiscussions
		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "abc123",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "First discussion"},
					},
				},
				{
					ID: "ghi789",
					Notes: []*gitlab.Note{
						{ID: 300, Body: "Third discussion"},
					},
				},
			}, nil, nil)

		// Mock ResolveMergeRequestDiscussion with Resolved: false
		unresolved := false
		testClient.MockDiscussions.EXPECT().
			ResolveMergeRequestDiscussion("OWNER/REPO", int64(1), "ghi789", gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discussionID string, opts *gitlab.ResolveMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, &unresolved, opts.Resolved)
				return &gitlab.Discussion{ID: "ghi789"}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --unresolve 300`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "✓ Discussion unresolved (note #300 in !1)")
	})
}

func Test_mrNote_diffComment(t *testing.T) {
	t.Parallel()

	diffContent := `@@ -1,5 +1,6 @@
 line1
-old line2
+new line2
+added line3
 line4
 line5
`

	makeMR := func(t *testing.T, testClient *gitlabtesting.TestClient) {
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

	makeDiffVersion := func(t *testing.T, testClient *gitlabtesting.TestClient, diffStr string) {
		t.Helper()
		testClient.MockMergeRequests.EXPECT().
			GetMergeRequestDiffVersions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.MergeRequestDiffVersion{
				{ID: 10, BaseCommitSHA: "base", HeadCommitSHA: "head", StartCommitSHA: "start"},
			}, nil, nil)

		testClient.MockMergeRequests.EXPECT().
			GetSingleMergeRequestDiffVersion("OWNER/REPO", int64(1), int64(10), gomock.Any()).
			Return(&gitlab.MergeRequestDiffVersion{
				ID:             10,
				BaseCommitSHA:  "base",
				HeadCommitSHA:  "head",
				StartCommitSHA: "start",
				Diffs: []*gitlab.Diff{
					{
						NewPath: "main.go",
						OldPath: "main.go",
						Diff:    diffStr,
					},
				},
			}, nil, nil)
	}

	t.Run("diff comment on new-side line", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)
		makeDiffVersion(t, testClient, diffContent)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "Comment on new line", *opts.Body)
				require.NotNil(t, opts.Position)
				assert.Equal(t, "text", *opts.Position.PositionType)
				assert.Equal(t, "main.go", *opts.Position.NewPath)
				assert.Equal(t, int64(2), *opts.Position.NewLine)
				assert.Equal(t, "base", *opts.Position.BaseSHA)
				assert.Equal(t, "head", *opts.Position.HeadSHA)
				assert.Equal(t, "start", *opts.Position.StartSHA)
				return &gitlab.Discussion{
					ID: "disc-diff-1",
					Notes: []*gitlab.Note{
						{ID: 500},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --file main.go --line 2 -m "Comment on new line"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_500")
	})

	t.Run("diff comment on old-side line", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)
		makeDiffVersion(t, testClient, diffContent)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "Comment on removed line", *opts.Body)
				require.NotNil(t, opts.Position)
				assert.Equal(t, int64(2), *opts.Position.OldLine)
				return &gitlab.Discussion{
					ID: "disc-diff-2",
					Notes: []*gitlab.Note{
						{ID: 501},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --file main.go --old-line 2 -m "Comment on removed line"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_501")
	})

	t.Run("diff comment with multiline range", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)
		makeDiffVersion(t, testClient, diffContent)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				require.NotNil(t, opts.Position)
				require.NotNil(t, opts.Position.LineRange)
				assert.NotNil(t, opts.Position.LineRange.Start)
				assert.NotNil(t, opts.Position.LineRange.End)
				return &gitlab.Discussion{
					ID: "disc-diff-3",
					Notes: []*gitlab.Note{
						{ID: 502},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --file main.go --line 2:3 -m "Range comment"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_502")
	})

	t.Run("file-level diff comment (no line)", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)
		makeDiffVersion(t, testClient, diffContent)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				require.NotNil(t, opts.Position)
				assert.Equal(t, "text", *opts.Position.PositionType)
				return &gitlab.Discussion{
					ID: "disc-diff-4",
					Notes: []*gitlab.Note{
						{ID: 503},
					},
				}, nil, nil
			})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		output, err := exec(`1 --file main.go -m "File-level comment"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_503")
	})

	t.Run("file not found in diff", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)
		makeDiffVersion(t, testClient, diffContent)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 --file nonexistent.go --line 1 -m "bad file"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in MR diff")
	})

	t.Run("--file and --resolve are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 --file main.go --resolve 100 -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "if any flags in the group [file resolve] are set none of the others can be")
	})

	t.Run("--line and --old-line are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 --file main.go --line 5 --old-line 3 -m "test"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "if any flags in the group [line old-line] are set none of the others can be")
	})

	t.Run("invalid line format", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)
		makeMR(t, testClient)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdNote(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
		)

		_, err := exec(`1 --file main.go --line abc -m "bad line"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid line number")
	})
}
