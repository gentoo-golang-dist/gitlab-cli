//go:build !integration

package note

import (
	"errors"
	"net/http"
	"testing"

	"git.sr.ht/~timofurrer/ugh"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_NewCmdCreate(t *testing.T) {
	t.Parallel()

	t.Run("--message flag specified", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "Here is my note", *opts.Body)
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		exec := setupCreateExec(t, testClient)

		output, err := exec(`1 --message "Here is my note"`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Equal(t, "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301\n", output.String())
	})

	t.Run("merge request not found", func(t *testing.T) {
		t.Parallel()

		testClient := setupMRNotFound(t)

		exec := setupCreateExec(t, testClient)

		_, err := exec(`122`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Not Found")
	})
}

func Test_NewCmdCreate_error(t *testing.T) {
	t.Parallel()

	t.Run("note could not be created", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		unauthorizedResp := &gitlab.Response{
			Response: &http.Response{StatusCode: http.StatusUnauthorized},
		}
		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return(nil, unauthorizedResp, errors.New("401 Unauthorized"))

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 -m "Some message"`)
		require.Error(t, err)
	})
}

func Test_cmdCreate_prompt(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	t.Run("message provided via prompt", func(t *testing.T) {
		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Contains(t, *opts.Body, "some note message")
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		c := ugh.New(t)
		c.Expect(ugh.Input("Note message:")).
			Do(ugh.Type("some note message"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdCreate(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithConsole(t, c),
		)

		output, err := exec(`1`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_301")
	})

	t.Run("message is empty", func(t *testing.T) {
		testClient := setupMR(t)

		c := ugh.New(t)
		c.Expect(ugh.Input("Note message:")).
			Do(ugh.Type(""))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdCreate(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithConsole(t, c),
		)

		_, err := exec(`1`)
		require.Error(t, err)
		assert.Equal(t, "aborted... Note has an empty message.", err.Error())
	})
}

func Test_cmdCreate_unique_prompt(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	t.Run("duplicate found", func(t *testing.T) {
		testClient := setupMR(t)

		testClient.MockNotes.EXPECT().
			ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Note{
				{ID: 0, Body: "aaa"},
				{ID: 111, Body: "bbb"},
				{ID: 222, Body: "some note message"},
				{ID: 333, Body: "ccc"},
			}, nil, nil)

		c := ugh.New(t)
		c.Expect(ugh.Input("Note message:")).
			Do(ugh.Type("some note message"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdCreate(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithConsole(t, c),
		)

		output, err := exec(`1 --unique`)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_222")
	})
}

func Test_cmdCreate_unique(t *testing.T) {
	t.Parallel()

	t.Run("no duplicate creates new note", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockNotes.EXPECT().
			ListMergeRequestNotes("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Note{
				{ID: 100, Body: "other note"},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
				assert.Equal(t, "brand new note", *opts.Body)
				return &gitlab.Discussion{
					ID: "disc1",
					Notes: []*gitlab.Note{
						{ID: 301, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
					},
				}, nil, nil
			})

		exec := setupCreateExec(t, testClient)

		output, err := exec(`1 -m "brand new note" --unique`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_301")
	})
}

func Test_cmdCreate_reply(t *testing.T) {
	t.Parallel()

	const fullID = "abc12345deadbeef1234567890abcdef12345678"

	t.Run("replies to discussion by full ID", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{{ID: fullID}}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			AddMergeRequestDiscussionNote("OWNER/REPO", int64(1), fullID, gomock.Any(), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discID string, opts *gitlab.AddMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "I agree!", *opts.Body)
				return &gitlab.Note{ID: 901}, nil, nil
			})

		exec := setupCreateExec(t, testClient)

		output, err := exec(`1 --reply ` + fullID + ` -m "I agree!"`)
		require.NoError(t, err)
		assert.Equal(t, "https://gitlab.com/OWNER/REPO/merge_requests/1#note_901\n", output.String())
	})

	t.Run("replies to discussion by prefix", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{
				{ID: fullID},
				{ID: "ff000000deadbeef1234567890abcdef12345678"},
			}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			AddMergeRequestDiscussionNote("OWNER/REPO", int64(1), fullID, gomock.Any(), gomock.Any()).
			Return(&gitlab.Note{ID: 902}, nil, nil)

		exec := setupCreateExec(t, testClient)

		output, err := exec(`1 --reply abc12345 -m "thanks"`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "#note_902")
	})

	t.Run("prefix too short", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 --reply abc -m "hi"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 8 characters")
	})

	t.Run("prefix not found", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{{ID: "ff000000deadbeef1234567890abcdef12345678"}}, nil, nil)

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 --reply abc12345 -m "hi"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no discussion found")
	})

	t.Run("ambiguous prefix", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{
				{ID: "abc123450000beef1234567890abcdef12345678"},
				{ID: "abc123459999beef1234567890abcdef12345678"},
			}, nil, nil)

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 --reply abc12345 -m "hi"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "matches 2 discussions")
	})

	t.Run("AddMergeRequestDiscussionNote error wrapped", func(t *testing.T) {
		t.Parallel()

		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{{ID: fullID}}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			AddMergeRequestDiscussionNote("OWNER/REPO", int64(1), fullID, gomock.Any(), gomock.Any()).
			Return(nil, nil, errors.New("boom"))

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 --reply ` + fullID + ` -m "hi"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add reply")
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("--reply and --unique are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		testClient := gitlabtesting.NewTestClient(t)

		exec := setupCreateExec(t, testClient)

		_, err := exec(`1 --reply abc12345 --unique -m "hi"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "none of the others can be")
	})
}

func Test_cmdCreate_reply_prompt(t *testing.T) {
	// NOTE: This test cannot run in parallel because the huh form library
	// uses global state (charmbracelet/bubbles runeutil sanitizer).

	const fullID = "abc12345deadbeef1234567890abcdef12345678"

	t.Run("message provided via prompt", func(t *testing.T) {
		testClient := setupMR(t)

		testClient.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
			Return([]*gitlab.Discussion{{ID: fullID}}, nil, nil)

		testClient.MockDiscussions.EXPECT().
			AddMergeRequestDiscussionNote("OWNER/REPO", int64(1), fullID, gomock.Any(), gomock.Any()).
			DoAndReturn(func(pid any, mrIID int64, discID string, opts *gitlab.AddMergeRequestDiscussionNoteOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Note, *gitlab.Response, error) {
				assert.Equal(t, "prompted reply", *opts.Body)
				return &gitlab.Note{ID: 950}, nil, nil
			})

		c := ugh.New(t)
		c.Expect(ugh.Input("Note message:")).
			Do(ugh.Type("prompted reply"))

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdCreate(f)
		}, true,
			cmdtest.WithGitLabClient(testClient.Client),
			cmdtest.WithBaseRepo("OWNER", "REPO", ""),
			cmdtest.WithConfig(config.NewFromString("editor: vi")),
			cmdtest.WithConsole(t, c),
		)

		output, err := exec(`1 --reply ` + fullID)
		require.NoError(t, err)
		assert.Empty(t, output.Stderr())
		assert.Contains(t, output.String(), "https://gitlab.com/OWNER/REPO/merge_requests/1#note_950")
	})
}

func Test_cmdCreate_stdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		stdin       string
		wantErr     string
		expectedOut string
	}{
		{
			name:        "reads body from stdin when not a TTY",
			stdin:       "Message from stdin\n",
			expectedOut: "https://gitlab.com/OWNER/REPO/merge_requests/1#note_700\n",
		},
		{
			name:    "empty stdin produces error",
			stdin:   "",
			wantErr: "aborted... Note has an empty message.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testClient := setupMR(t)

			if tt.wantErr == "" {
				testClient.MockDiscussions.EXPECT().
					CreateMergeRequestDiscussion("OWNER/REPO", int64(1), gomock.Any(), gomock.Any()).
					DoAndReturn(func(pid any, mrIID int64, opts *gitlab.CreateMergeRequestDiscussionOptions, options ...gitlab.RequestOptionFunc) (*gitlab.Discussion, *gitlab.Response, error) {
						assert.Equal(t, "Message from stdin", *opts.Body)
						return &gitlab.Discussion{
							ID: "disc-stdin",
							Notes: []*gitlab.Note{
								{ID: 700, NoteableID: 1, NoteableType: "MergeRequest", NoteableIID: 1},
							},
						}, nil, nil
					})
			}

			exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
				return NewCmdCreate(f)
			}, false,
				cmdtest.WithGitLabClient(testClient.Client),
				cmdtest.WithBaseRepo("OWNER", "REPO", ""),
				cmdtest.WithConfig(config.NewFromString("editor: vi")),
				cmdtest.WithStdin(tt.stdin),
			)

			output, err := exec(`1`)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOut, output.String())
		})
	}
}

// --- test helpers ---

func setupMR(t *testing.T) *gitlabtesting.TestClient {
	t.Helper()
	testClient := gitlabtesting.NewTestClient(t)
	mockMR1(t, testClient)
	return testClient
}

func setupMRNotFound(t *testing.T) *gitlabtesting.TestClient {
	t.Helper()
	testClient := gitlabtesting.NewTestClient(t)
	notFoundResp := &gitlab.Response{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(122), gomock.Any()).
		Return(nil, notFoundResp, gitlab.ErrNotFound)
	return testClient
}

func setupCreateExec(t *testing.T, testClient *gitlabtesting.TestClient) cmdtest.CmdExecFunc {
	t.Helper()
	return cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
		return NewCmdCreate(f)
	}, true,
		cmdtest.WithGitLabClient(testClient.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithConfig(config.NewFromString("editor: vi")),
	)
}
