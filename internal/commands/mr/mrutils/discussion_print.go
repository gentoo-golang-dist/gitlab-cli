package mrutils

import (
	"fmt"
	"io"
	"sort"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	issuableView "gitlab.com/gitlab-org/cli/internal/commands/issuable/view"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

// PrintDiscussionsTTY renders discussions in TTY format to the given writer.
func PrintDiscussionsTTY(out io.Writer, ios *iostreams.IOStreams, discussions []*gitlab.Discussion, showSystemLogs bool) {
	c := ios.Color()

	for _, discussion := range discussions {
		if len(discussion.Notes) == 0 {
			continue
		}

		firstNote := discussion.Notes[0]

		// Skip system notes unless showSystemLogs is set
		if firstNote.System && !showSystemLogs {
			continue
		}

		// Threaded discussions (not individual notes)
		if !discussion.IndividualNote && len(discussion.Notes) > 1 {
			// Print thread header with first note ID
			fmt.Fprintf(out, "Thread [#%d]", firstNote.ID)

			// Show resolution status if resolvable
			if firstNote.Resolvable {
				if firstNote.Resolved {
					fmt.Fprint(out, c.Green(" ✓ resolved"))
				} else {
					fmt.Fprint(out, c.Yellow(" ⚠ unresolved"))
				}
			}
			fmt.Fprintln(out)

			// Print first note
			createdAt := utils.TimeToPrettyTimeAgo(*firstNote.CreatedAt)
			fmt.Fprintf(out, "  @%s commented ", firstNote.Author.Username)
			fmt.Fprintln(out, c.Gray(createdAt))

			if firstNote.Position != nil {
				PrintCommentFileContext(out, c, firstNote.Position)
			}

			body, _ := utils.RenderMarkdown(firstNote.Body, ios.BackgroundColor())
			fmt.Fprintln(out, utils.Indent(body, "  "))
			fmt.Fprintln(out)

			// Print replies (indented)
			for i, note := range discussion.Notes[1:] {
				if note.System && !showSystemLogs {
					continue
				}
				replyTime := utils.TimeToPrettyTimeAgo(*note.CreatedAt)
				fmt.Fprintf(out, "    @%s replied ", note.Author.Username)
				fmt.Fprintln(out, c.Gray(replyTime))

				replyBody, _ := utils.RenderMarkdown(note.Body, ios.BackgroundColor())
				fmt.Fprintln(out, utils.Indent(replyBody, "    "))
				if i < len(discussion.Notes[1:])-1 {
					fmt.Fprintln(out)
				}
			}
			fmt.Fprintln(out)
		} else {
			// Individual note (not a thread)
			note := firstNote
			createdAt := utils.TimeToPrettyTimeAgo(*note.CreatedAt)
			fmt.Fprint(out, "@", note.Author.Username)
			if note.System {
				fmt.Fprintf(out, " %s ", note.Body)
				fmt.Fprintln(out, c.Gray(createdAt))
			} else {
				body, _ := utils.RenderMarkdown(note.Body, ios.BackgroundColor())
				fmt.Fprint(out, " commented ")
				fmt.Fprintf(out, c.Gray("%s\n"), createdAt)

				if note.Position != nil {
					PrintCommentFileContext(out, c, note.Position)
				}

				fmt.Fprintln(out, utils.Indent(body, " "))
			}
			fmt.Fprintln(out)
		}
	}
}

// PrintDiscussionsRaw renders discussions as flat, chronologically sorted notes in raw format.
func PrintDiscussionsRaw(out io.Writer, discussions []*gitlab.Discussion, showSystemLogs bool) {
	var notes []*gitlab.Note
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.System && !showSystemLogs {
				continue
			}
			notes = append(notes, note)
		}
	}

	// Sort notes chronologically by creation time
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.Before(*notes[j].CreatedAt)
	})

	fmt.Fprint(out, issuableView.RawIssuableNotes(notes, true, showSystemLogs, "merge request"))
}

// PrintCommentFileContext prints file and line context for a note position.
func PrintCommentFileContext(out io.Writer, c *iostreams.ColorPalette, pos *gitlab.NotePosition) {
	// Check for multi-line comment first
	if pos.LineRange != nil && pos.LineRange.StartRange != nil && pos.LineRange.EndRange != nil {
		startLine := pos.LineRange.StartRange.NewLine
		endLine := pos.LineRange.EndRange.NewLine

		// Fall back to old line numbers if new ones aren't available
		if startLine == 0 {
			startLine = pos.LineRange.StartRange.OldLine
		}
		if endLine == 0 {
			endLine = pos.LineRange.EndRange.OldLine
		}

		// Display range if we have valid start and end lines
		if startLine > 0 && endLine > 0 {
			filePath := pos.NewPath
			if filePath == "" {
				filePath = pos.OldPath
			}
			if filePath != "" {
				if startLine != endLine {
					fmt.Fprintf(out, " on %s:%d-%d\n", c.Cyan(filePath), startLine, endLine)
				} else {
					fmt.Fprintf(out, " on %s:%d\n", c.Cyan(filePath), startLine)
				}
				return
			}
		}
	}

	// Fall back to single-line comment
	if pos.NewPath != "" && pos.NewLine > 0 {
		fmt.Fprintf(out, " on %s:%d\n", c.Cyan(pos.NewPath), pos.NewLine)
	} else if pos.OldPath != "" && pos.OldLine > 0 {
		fmt.Fprintf(out, " on %s:%d\n", c.Cyan(pos.OldPath), pos.OldLine)
	}
}
