//go:build !integration

package mrutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func Test_ParseLine(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantStart int
		wantEnd   int
		wantErr   string
	}{
		{name: "empty string", input: "", wantStart: 0, wantEnd: 0},
		{name: "single line", input: "42", wantStart: 42, wantEnd: 42},
		{name: "line range", input: "10:15", wantStart: 10, wantEnd: 15},
		{name: "same start and end", input: "5:5", wantStart: 5, wantEnd: 5},
		{name: "invalid number", input: "abc", wantErr: `invalid line number "abc"`},
		{name: "invalid range end", input: "10:abc", wantErr: `invalid line range "10:abc"`},
		{name: "reversed range", input: "15:10", wantErr: `invalid line range "15:10": end must be >= start`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParseLine(tt.input)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			}
		})
	}
}

func Test_FindFileDiff(t *testing.T) {
	version := &gitlab.MergeRequestDiffVersion{
		Diffs: []*gitlab.Diff{
			{NewPath: "src/main.go", OldPath: "src/main.go"},
			{NewPath: "src/new.go", OldPath: "src/old.go"},
		},
	}

	t.Run("match by NewPath", func(t *testing.T) {
		d, err := FindFileDiff(version, "src/main.go")
		require.NoError(t, err)
		assert.Equal(t, "src/main.go", d.NewPath)
	})

	t.Run("match by OldPath", func(t *testing.T) {
		d, err := FindFileDiff(version, "src/old.go")
		require.NoError(t, err)
		assert.Equal(t, "src/new.go", d.NewPath)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := FindFileDiff(version, "nonexistent.go")
		assert.EqualError(t, err, `file "nonexistent.go" not found in MR diff`)
	})
}

func Test_BuildDiffPosition(t *testing.T) {
	version := &gitlab.MergeRequestDiffVersion{
		BaseCommitSHA:  "base123",
		HeadCommitSHA:  "head456",
		StartCommitSHA: "start789",
	}

	// Simple diff: one removed, one added, one context line
	diffContent := `@@ -1,3 +1,3 @@
 unchanged line
-old line
+new line
 another unchanged
`

	fileDiff := &gitlab.Diff{
		NewPath: "file.go",
		OldPath: "file.go",
		Diff:    diffContent,
	}

	t.Run("new-side single line on added line", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 2, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, "base123", *pos.BaseSHA)
		assert.Equal(t, "head456", *pos.HeadSHA)
		assert.Equal(t, "start789", *pos.StartSHA)
		assert.Equal(t, "file.go", *pos.NewPath)
		assert.Equal(t, int64(2), *pos.NewLine)
		// Added line has no OldLine
		assert.Nil(t, pos.OldLine)
		assert.Nil(t, pos.LineRange)
	})

	t.Run("new-side single line on unchanged line", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 1, 1, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), *pos.NewLine)
		assert.Equal(t, int64(1), *pos.OldLine) // Unchanged => both sides
		assert.Nil(t, pos.LineRange)
	})

	t.Run("new-side multiline range", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 1, 3, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), *pos.NewLine)
		require.NotNil(t, pos.LineRange)
		assert.Equal(t, "new", *pos.LineRange.Start.Type)
		assert.Equal(t, "new", *pos.LineRange.End.Type)
	})

	t.Run("old-side line on removed line", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 0, 0, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(2), *pos.OldLine)
		assert.Nil(t, pos.NewLine) // Removed line => no new side
	})

	t.Run("old-side line on unchanged line", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 0, 0, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), *pos.OldLine)
		assert.Equal(t, int64(1), *pos.NewLine) // Unchanged => both sides
	})

	t.Run("file-level comment", func(t *testing.T) {
		pos, err := BuildDiffPosition(version, fileDiff, 0, 0, 0)
		require.NoError(t, err)
		// Should target first line in diff
		require.NotNil(t, pos.NewLine)
	})

	t.Run("line not in diff", func(t *testing.T) {
		_, err := BuildDiffPosition(version, fileDiff, 999, 999, 0)
		assert.ErrorContains(t, err, "line 999 not found in diff")
	})

	t.Run("old line not in diff", func(t *testing.T) {
		_, err := BuildDiffPosition(version, fileDiff, 0, 0, 999)
		assert.ErrorContains(t, err, "old line 999 not found in diff")
	})
}

func Test_lineCode(t *testing.T) {
	code := lineCode("file.go", 42)
	// lineCode should be deterministic
	assert.Equal(t, lineCode("file.go", 42), code)
	// Different file should produce different code
	assert.NotEqual(t, lineCode("other.go", 42), code)
	// Different line should produce different code
	assert.NotEqual(t, lineCode("file.go", 43), code)
	// Should contain the line number
	assert.Contains(t, code, "_0_42")
}

func Test_ResolveDiscussionID(t *testing.T) {
	// Save and restore original
	orig := ResolveDiscussionID
	origList := ListAllDiscussions
	t.Cleanup(func() {
		ResolveDiscussionID = orig
		ListAllDiscussions = origList
	})

	discussions := []*gitlab.Discussion{
		{ID: "abcdef1234567890abcdef1234567890abcdef12"},
		{ID: "abcdef1234567890abcdef1234567890xxxxxxxx"},
		{ID: "12345678abcdef90abcdef1234567890abcdef12"},
	}

	ListAllDiscussions = func(_ *gitlab.Client, _ string, _ int64) ([]*gitlab.Discussion, error) {
		return discussions, nil
	}

	// Reset to original implementation for these tests
	ResolveDiscussionID = orig

	t.Run("prefix too short", func(t *testing.T) {
		_, err := ResolveDiscussionID(nil, "proj", 1, "abcdef1")
		assert.ErrorContains(t, err, "at least 8 characters")
	})

	t.Run("unique match", func(t *testing.T) {
		id, err := ResolveDiscussionID(nil, "proj", 1, "12345678")
		require.NoError(t, err)
		assert.Equal(t, "12345678abcdef90abcdef1234567890abcdef12", id)
	})

	t.Run("ambiguous match", func(t *testing.T) {
		_, err := ResolveDiscussionID(nil, "proj", 1, "abcdef12")
		assert.ErrorContains(t, err, "matches 2 discussions")
	})

	t.Run("no match", func(t *testing.T) {
		_, err := ResolveDiscussionID(nil, "proj", 1, "ffffffff")
		assert.ErrorContains(t, err, "no discussion found")
	})

	t.Run("full ID match", func(t *testing.T) {
		id, err := ResolveDiscussionID(nil, "proj", 1, "abcdef1234567890abcdef1234567890abcdef12")
		require.NoError(t, err)
		assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", id)
	})
}

func Test_FindNoteInDiscussions(t *testing.T) {
	orig := FindNoteInDiscussions
	origList := ListAllDiscussions
	t.Cleanup(func() {
		FindNoteInDiscussions = orig
		ListAllDiscussions = origList
	})

	discussions := []*gitlab.Discussion{
		{
			ID: "disc1",
			Notes: []*gitlab.Note{
				{ID: 100, Body: "first note"},
				{ID: 101, Body: "reply"},
			},
		},
		{
			ID: "disc2",
			Notes: []*gitlab.Note{
				{ID: 200, Body: "second discussion"},
			},
		},
	}

	ListAllDiscussions = func(_ *gitlab.Client, _ string, _ int64) ([]*gitlab.Discussion, error) {
		return discussions, nil
	}
	FindNoteInDiscussions = orig

	t.Run("finds note in first discussion", func(t *testing.T) {
		discID, note, err := FindNoteInDiscussions(nil, "proj", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, "disc1", discID)
		assert.Equal(t, "first note", note.Body)
	})

	t.Run("finds note in second discussion", func(t *testing.T) {
		discID, note, err := FindNoteInDiscussions(nil, "proj", 1, 200)
		require.NoError(t, err)
		assert.Equal(t, "disc2", discID)
		assert.Equal(t, "second discussion", note.Body)
	})

	t.Run("finds reply note", func(t *testing.T) {
		discID, note, err := FindNoteInDiscussions(nil, "proj", 1, 101)
		require.NoError(t, err)
		assert.Equal(t, "disc1", discID)
		assert.Equal(t, "reply", note.Body)
	})

	t.Run("note not found", func(t *testing.T) {
		_, _, err := FindNoteInDiscussions(nil, "proj", 1, 999)
		assert.ErrorContains(t, err, "note 999 not found")
	})
}
