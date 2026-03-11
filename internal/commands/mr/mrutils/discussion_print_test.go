//go:build !integration

package mrutils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_PrintCommentFileContext(t *testing.T) {
	// NOTE: we need to force disable colors, otherwise we'd need ANSI sequences in our test output assertions.
	t.Setenv("NO_COLOR", "true")

	ioStr, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
	c := ioStr.Color()

	tests := []struct {
		name     string
		note     *gitlab.Note
		expected string
	}{
		{
			name: "single line comment on new file",
			note: &gitlab.Note{
				Position: &gitlab.NotePosition{
					NewPath: "internal/commands/mr/view/mr_view.go",
					NewLine: 42,
				},
			},
			expected: " on internal/commands/mr/view/mr_view.go:42\n",
		},
		{
			name: "single line comment on old file",
			note: &gitlab.Note{
				Position: &gitlab.NotePosition{
					OldPath: "internal/commands/mr/view/mr_view.go",
					OldLine: 35,
				},
			},
			expected: " on internal/commands/mr/view/mr_view.go:35\n",
		},
		{
			name: "multi-line comment",
			note: &gitlab.Note{
				Position: &gitlab.NotePosition{
					NewPath: "internal/gateway/mcp/tools/get_coin_open_interest.go",
					LineRange: &gitlab.LineRange{
						StartRange: &gitlab.LinePosition{NewLine: 63},
						EndRange:   &gitlab.LinePosition{NewLine: 68},
					},
				},
			},
			expected: " on internal/gateway/mcp/tools/get_coin_open_interest.go:63-68\n",
		},
		{
			name: "single line range (same start and end)",
			note: &gitlab.Note{
				Position: &gitlab.NotePosition{
					NewPath: "main.go",
					LineRange: &gitlab.LineRange{
						StartRange: &gitlab.LinePosition{NewLine: 10},
						EndRange:   &gitlab.LinePosition{NewLine: 10},
					},
				},
			},
			expected: " on main.go:10\n",
		},
		{
			name: "position with no line numbers",
			note: &gitlab.Note{
				Position: &gitlab.NotePosition{
					NewPath: "file.go",
					NewLine: 0,
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintCommentFileContext(&buf, c, tt.note.Position)
			got := buf.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}
