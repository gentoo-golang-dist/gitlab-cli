//go:build !integration

package note

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func makeMRForList(t *testing.T, tc *gitlabtesting.TestClient) {
	t.Helper()
	tc.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(1), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:     1,
				IID:    1,
				WebURL: "https://gitlab.com/OWNER/REPO/merge_requests/1",
			},
		}, nil, nil)
}

func setupListCmd(t *testing.T, tc *gitlabtesting.TestClient) cmdtest.CmdExecFunc {
	t.Helper()
	return cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
		return NewCmdNote(f)
	}, true,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", ""),
		cmdtest.WithConfig(config.NewFromString("editor: vi")),
	)
}

func ts(s string) *time.Time {
	t, _ := time.Parse(time.DateTime, s)
	return &t
}

func Test_NoteList(t *testing.T) {
	t.Parallel()

	t.Run("lists all discussions", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "abcdef1234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID:        100,
							Body:      "General comment",
							Author:    gitlab.NoteAuthor{Username: "alice"},
							CreatedAt: ts("2025-01-15 10:30:00"),
						},
					},
				},
				{
					ID: "12345678abcdef90abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID:         200,
							Body:       "Diff note",
							Author:     gitlab.NoteAuthor{Username: "bob"},
							CreatedAt:  ts("2025-01-15 11:00:00"),
							Position:   &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 42},
							Resolvable: true,
							Resolved:   false,
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "#abcdef12")
		assert.Contains(t, out, "(general)")
		assert.Contains(t, out, "@alice")
		assert.Contains(t, out, "General comment")
		assert.Contains(t, out, "#12345678")
		assert.Contains(t, out, "[UNRESOLVED]")
		assert.Contains(t, out, "main.go:42")
		assert.Contains(t, out, "(diff)")
		assert.Contains(t, out, "@bob")
	})

	t.Run("no discussions found", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "No discussions found.")
	})

	t.Run("filter by diff", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "general1234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "General", Author: gitlab.NoteAuthor{Username: "alice"}},
					},
				},
				{
					ID: "diffnote234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "Diff", Author: gitlab.NoteAuthor{Username: "bob"},
							Position: &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 5},
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --filter diff`)
		require.NoError(t, err)

		out := output.String()
		assert.NotContains(t, out, "#general1")
		assert.Contains(t, out, "#diffnote")
		assert.Contains(t, out, "(diff)")
	})

	t.Run("filter by general", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "general1234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "General", Author: gitlab.NoteAuthor{Username: "alice"}},
					},
				},
				{
					ID: "diffnote234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "Diff", Author: gitlab.NoteAuthor{Username: "bob"},
							Position: &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 5},
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --filter general`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "#general1")
		assert.NotContains(t, out, "#diffnote")
	})

	t.Run("filter by system", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "general1234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "General", Author: gitlab.NoteAuthor{Username: "alice"}},
					},
				},
				{
					ID: "systemnote34567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 300, Body: "merged", Author: gitlab.NoteAuthor{Username: "bot"}, System: true},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --filter system`)
		require.NoError(t, err)

		out := output.String()
		assert.NotContains(t, out, "#general1")
		assert.Contains(t, out, "#systemno")
		assert.Contains(t, out, "(system)")
	})

	t.Run("state resolved", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "resolved234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "Done", Author: gitlab.NoteAuthor{Username: "alice"},
							Resolvable: true, Resolved: true,
						},
					},
				},
				{
					ID: "unresolv234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "TODO", Author: gitlab.NoteAuthor{Username: "bob"},
							Resolvable: true, Resolved: false,
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --state resolved`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "#resolved")
		assert.Contains(t, out, "[RESOLVED]")
		assert.NotContains(t, out, "#unresolv")
	})

	t.Run("state unresolved", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "resolved234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "Done", Author: gitlab.NoteAuthor{Username: "alice"},
							Resolvable: true, Resolved: true,
						},
					},
				},
				{
					ID: "unresolv234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "TODO", Author: gitlab.NoteAuthor{Username: "bob"},
							Resolvable: true, Resolved: false,
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --state unresolved`)
		require.NoError(t, err)

		out := output.String()
		assert.NotContains(t, out, "#resolved")
		assert.Contains(t, out, "#unresolv")
		assert.Contains(t, out, "[UNRESOLVED]")
	})

	t.Run("filter by file", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "filemain234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "Comment on main", Author: gitlab.NoteAuthor{Username: "alice"},
							Position: &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 10},
						},
					},
				},
				{
					ID: "fileutil234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "Comment on utils", Author: gitlab.NoteAuthor{Username: "bob"},
							Position: &gitlab.NotePosition{NewPath: "utils.go", OldPath: "utils.go", NewLine: 5},
						},
					},
				},
				{
					ID: "generalx234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 300, Body: "General note", Author: gitlab.NoteAuthor{Username: "carol"}},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --file main.go`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "#filemain")
		assert.NotContains(t, out, "#fileutil")
		assert.NotContains(t, out, "#generalx")
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "jsontest234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 100, Body: "Hello", Author: gitlab.NoteAuthor{Username: "alice"}},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --json`)
		require.NoError(t, err)

		var parsed []map[string]any
		err = json.Unmarshal([]byte(output.String()), &parsed)
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		assert.Equal(t, "jsontest234567890abcdef1234567890abcdef12", parsed[0]["id"])
	})

	t.Run("old-side line display", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "oldline12345678901234567890abcdef12345678",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "Removed line", Author: gitlab.NoteAuthor{Username: "alice"},
							Position: &gitlab.NotePosition{NewPath: "", OldPath: "old.go", OldLine: 7},
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "old.go:~7")
	})

	t.Run("long body is truncated", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		longBody := strings.Repeat("x", 250)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "longbody234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{ID: 100, Body: longBody, Author: gitlab.NoteAuthor{Username: "alice"}},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1`)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "...")
	})

	t.Run("multiple notes in a thread", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "thread12345678901234567890abcdef12345678",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "First note", Author: gitlab.NoteAuthor{Username: "alice"},
							CreatedAt: ts("2025-01-15 10:00:00"),
						},
						{
							ID: 101, Body: "Reply", Author: gitlab.NoteAuthor{Username: "bob"},
							CreatedAt: ts("2025-01-15 10:05:00"),
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1`)
		require.NoError(t, err)

		out := output.String()
		assert.Contains(t, out, "@alice")
		assert.Contains(t, out, "First note")
		assert.Contains(t, out, "@bob")
		assert.Contains(t, out, "Reply")
	})

	t.Run("combined filters", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "diffresol234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "Resolved diff", Author: gitlab.NoteAuthor{Username: "alice"},
							Position:   &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 10},
							Resolvable: true, Resolved: true,
						},
					},
				},
				{
					ID: "diffunres234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "Unresolved diff", Author: gitlab.NoteAuthor{Username: "bob"},
							Position:   &gitlab.NotePosition{NewPath: "main.go", OldPath: "main.go", NewLine: 20},
							Resolvable: true, Resolved: false,
						},
					},
				},
				{
					ID: "genunres234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 300, Body: "Unresolved general", Author: gitlab.NoteAuthor{Username: "carol"},
							Resolvable: true, Resolved: false,
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --filter diff --state unresolved`)
		require.NoError(t, err)

		out := output.String()
		assert.NotContains(t, out, "#diffreso") // resolved, excluded
		assert.Contains(t, out, "#diffunre")    // unresolved diff, included
		assert.NotContains(t, out, "#genunres") // general, excluded by filter
	})

	t.Run("non-resolvable excluded from state filter", func(t *testing.T) {
		t.Parallel()

		tc := gitlabtesting.NewTestClient(t)
		makeMRForList(t, tc)

		tc.MockDiscussions.EXPECT().
			ListMergeRequestDiscussions("OWNER/REPO", int64(1), gomock.Any()).
			Return([]*gitlab.Discussion{
				{
					ID: "nonresol234567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 100, Body: "System note", Author: gitlab.NoteAuthor{Username: "bot"},
							System: true, Resolvable: false,
						},
					},
				},
				{
					ID: "resolvabl34567890abcdef1234567890abcdef12",
					Notes: []*gitlab.Note{
						{
							ID: 200, Body: "Unresolved", Author: gitlab.NoteAuthor{Username: "alice"},
							Resolvable: true, Resolved: false,
						},
					},
				},
			}, nil, nil)

		exec := setupListCmd(t, tc)
		output, err := exec(`list 1 --state unresolved`)
		require.NoError(t, err)

		out := output.String()
		assert.NotContains(t, out, "#nonresol") // non-resolvable, excluded
		assert.Contains(t, out, "#resolvab")    // resolvable + unresolved, included
	})
}
