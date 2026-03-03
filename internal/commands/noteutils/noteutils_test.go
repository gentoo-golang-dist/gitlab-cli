//go:build !integration

package noteutils

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestFileContext(t *testing.T) {
	tests := []struct {
		name     string
		position *gitlab.NotePosition
		expected fileContext
	}{
		{
			name:     "nil position",
			position: nil,
			expected: fileContext{},
		},
		{
			name: "single line comment on new file",
			position: &gitlab.NotePosition{
				NewPath: "src/main.go",
				NewLine: 42,
			},
			expected: fileContext{Path: "src/main.go", StartLine: 42, EndLine: 42},
		},
		{
			name: "single line comment on old file",
			position: &gitlab.NotePosition{
				OldPath: "src/main.go",
				OldLine: 35,
			},
			expected: fileContext{Path: "src/main.go", StartLine: 35, EndLine: 35},
		},
		{
			name: "multi-line comment with new lines",
			position: &gitlab.NotePosition{
				NewPath: "src/handler.go",
				LineRange: &gitlab.LineRange{
					StartRange: &gitlab.LinePosition{NewLine: 10},
					EndRange:   &gitlab.LinePosition{NewLine: 20},
				},
			},
			expected: fileContext{Path: "src/handler.go", StartLine: 10, EndLine: 20},
		},
		{
			name: "multi-line comment falling back to old lines",
			position: &gitlab.NotePosition{
				OldPath: "src/handler.go",
				LineRange: &gitlab.LineRange{
					StartRange: &gitlab.LinePosition{OldLine: 5},
					EndRange:   &gitlab.LinePosition{OldLine: 15},
				},
			},
			expected: fileContext{Path: "src/handler.go", StartLine: 5, EndLine: 15},
		},
		{
			name: "single line range (same start and end)",
			position: &gitlab.NotePosition{
				NewPath: "main.go",
				LineRange: &gitlab.LineRange{
					StartRange: &gitlab.LinePosition{NewLine: 10},
					EndRange:   &gitlab.LinePosition{NewLine: 10},
				},
			},
			expected: fileContext{Path: "main.go", StartLine: 10, EndLine: 10},
		},
		{
			name: "position with no line numbers",
			position: &gitlab.NotePosition{
				NewPath: "file.go",
				NewLine: 0,
			},
			expected: fileContext{},
		},
		{
			name: "line range with nil EndRange falls back to single line",
			position: &gitlab.NotePosition{
				NewPath: "src/main.go",
				NewLine: 7,
				LineRange: &gitlab.LineRange{
					StartRange: &gitlab.LinePosition{NewLine: 5},
					EndRange:   nil,
				},
			},
			expected: fileContext{Path: "src/main.go", StartLine: 7, EndLine: 7},
		},
		{
			name: "line range with nil StartRange falls back to single line",
			position: &gitlab.NotePosition{
				NewPath: "src/main.go",
				NewLine: 12,
				LineRange: &gitlab.LineRange{
					StartRange: nil,
					EndRange:   &gitlab.LinePosition{NewLine: 15},
				},
			},
			expected: fileContext{Path: "src/main.go", StartLine: 12, EndLine: 12},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileContext(tt.position)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRawNotes(t *testing.T) {
	time1, _ := time.Parse(time.RFC3339, "2023-03-09T16:50:20.111Z")
	time2, _ := time.Parse(time.RFC3339, "2023-03-09T16:52:30.222Z")

	fakeNote1 := &gitlab.Note{}
	fakeNote1.Author.Username = "bob"
	fakeNote2 := &gitlab.Note{}
	fakeNote2.Author.Username = "alice"

	t.Run("comments disabled returns empty", func(t *testing.T) {
		got := RawNotes([]*gitlab.Note{{Author: fakeNote1.Author}}, false, false, "issue")
		assert.Equal(t, "", got)
	})

	t.Run("no notes", func(t *testing.T) {
		got := RawNotes([]*gitlab.Note{}, true, false, "merge request")
		assert.Equal(t, "\n--\ncomments/notes:\n\nThere are no comments on this merge request.\n", got)
	})

	t.Run("plain comments without positions", func(t *testing.T) {
		notes := []*gitlab.Note{
			{
				System:    true,
				Author:    fakeNote1.Author,
				Body:      "assigned to @alice",
				CreatedAt: &time1,
			},
			{
				Author:    fakeNote1.Author,
				Body:      "Some comment",
				CreatedAt: &time1,
			},
			{
				Author:    fakeNote2.Author,
				Body:      "Another comment",
				CreatedAt: &time2,
			},
		}

		got := RawNotes(notes, true, true, "issue")
		want := strings.Join([]string{
			"\n--\ncomments/notes:\n",
			fmt.Sprintf("bob assigned to @alice %s", time1),
			"",
			fmt.Sprintf("bob commented %s", time1),
			"Some comment",
			"",
			fmt.Sprintf("alice commented %s", time2),
			"Another comment",
			"",
			"",
		}, "\n")

		assert.Equal(t, want, got)
	})

	t.Run("system notes hidden by default", func(t *testing.T) {
		notes := []*gitlab.Note{
			{
				System:    true,
				Author:    fakeNote1.Author,
				Body:      "assigned to @alice",
				CreatedAt: &time1,
			},
			{
				Author:    fakeNote1.Author,
				Body:      "Some comment",
				CreatedAt: &time1,
			},
		}

		got := RawNotes(notes, true, false, "issue")
		want := strings.Join([]string{
			"\n--\ncomments/notes:\n",
			fmt.Sprintf("bob commented %s", time1),
			"Some comment",
			"",
			"",
		}, "\n")

		assert.Equal(t, want, got)
	})

	t.Run("notes with diff positions", func(t *testing.T) {
		notes := []*gitlab.Note{
			{
				Author:    fakeNote1.Author,
				Body:      "Needs refactoring",
				CreatedAt: &time1,
				Position: &gitlab.NotePosition{
					NewPath: "src/main.go",
					NewLine: 42,
				},
			},
			{
				Author:    fakeNote2.Author,
				Body:      "Multi-line issue",
				CreatedAt: &time2,
				Position: &gitlab.NotePosition{
					NewPath: "src/handler.go",
					LineRange: &gitlab.LineRange{
						StartRange: &gitlab.LinePosition{NewLine: 10},
						EndRange:   &gitlab.LinePosition{NewLine: 20},
					},
				},
			},
			{
				Author:    fakeNote2.Author,
				Body:      "Plain comment",
				CreatedAt: &time2,
			},
		}

		got := RawNotes(notes, true, false, "merge request")
		want := strings.Join([]string{
			"\n--\ncomments/notes:\n",
			fmt.Sprintf("bob commented on src/main.go:42 %s", time1),
			"Needs refactoring",
			"",
			fmt.Sprintf("alice commented on src/handler.go:10-20 %s", time2),
			"Multi-line issue",
			"",
			fmt.Sprintf("alice commented %s", time2),
			"Plain comment",
			"",
			"",
		}, "\n")

		assert.Equal(t, want, got)
	})
}
