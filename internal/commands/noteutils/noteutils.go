package noteutils

import (
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type fileContext struct {
	Path      string
	StartLine int64
	EndLine   int64
}

func (fc fileContext) String() string {
	if fc.Path == "" {
		return ""
	}
	if fc.StartLine != fc.EndLine {
		return fmt.Sprintf("%s:%d-%d", fc.Path, fc.StartLine, fc.EndLine)
	}
	return fmt.Sprintf("%s:%d", fc.Path, fc.StartLine)
}

// FileContext extracts file path and line information from a note's diff position.
func FileContext(pos *gitlab.NotePosition) fileContext {
	if pos == nil {
		return fileContext{}
	}

	if pos.LineRange != nil && pos.LineRange.StartRange != nil && pos.LineRange.EndRange != nil {
		startLine := pos.LineRange.StartRange.NewLine
		endLine := pos.LineRange.EndRange.NewLine

		if startLine == 0 {
			startLine = pos.LineRange.StartRange.OldLine
		}
		if endLine == 0 {
			endLine = pos.LineRange.EndRange.OldLine
		}

		if startLine > 0 && endLine > 0 {
			filePath := pos.NewPath
			if filePath == "" {
				filePath = pos.OldPath
			}
			if filePath != "" {
				return fileContext{Path: filePath, StartLine: startLine, EndLine: endLine}
			}
		}
	}

	if pos.NewPath != "" && pos.NewLine > 0 {
		return fileContext{Path: pos.NewPath, StartLine: pos.NewLine, EndLine: pos.NewLine}
	}
	if pos.OldPath != "" && pos.OldLine > 0 {
		return fileContext{Path: pos.OldPath, StartLine: pos.OldLine, EndLine: pos.OldLine}
	}

	return fileContext{}
}

// RawNotes returns a list of comments/notes in a raw format.
// It handles both issuable notes and MR notes transparently —
// if a note has position data, the file context is included.
func RawNotes(notes []*gitlab.Note, showComments bool, showSystemLogs bool, entityName string) string {
	var out strings.Builder

	if !showComments {
		return ""
	}

	out.WriteString("\n--\ncomments/notes:\n\n")

	if len(notes) == 0 {
		out.WriteString(fmt.Sprintf("There are no comments on this %s.\n", entityName))
		return out.String()
	}

	for _, note := range notes {
		if note.System && !showSystemLogs {
			continue
		}

		if note.System {
			out.WriteString(fmt.Sprintf("%s %s %s\n\n", note.Author.Username, note.Body, note.CreatedAt.String()))
		} else {
			fc := FileContext(note.Position)
			if fc.Path != "" {
				out.WriteString(fmt.Sprintf("%s commented on %s %s\n%s\n\n", note.Author.Username, fc, note.CreatedAt.String(), note.Body))
			} else {
				out.WriteString(fmt.Sprintf("%s commented %s\n%s\n\n", note.Author.Username, note.CreatedAt.String(), note.Body))
			}
		}
	}

	return out.String()
}
