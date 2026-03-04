//go:build !integration

package ciutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatLogForLLM(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips ANSI codes",
			input:    "\033[32mSuccess\033[0m: all tests passed",
			expected: "Success: all tests passed",
		},
		{
			name:     "converts section markers to headings",
			input:    "\033[0Ksection_start:1234567890:build_step\r\033[0KBuilding project\ncompiling main.go\n\033[0Ksection_end:1234567890:build_step\r\033[0K",
			expected: "## Build step\nBuilding project\ncompiling main.go",
		},
		{
			name:     "removes progress bars",
			input:    "Downloading packages\n50%|████████░░░░░░░░| 5/10\n100%|████████████████| 10/10\nDone.",
			expected: "Downloading packages\nDone.",
		},
		{
			name:     "collapses blank lines",
			input:    "line 1\n\n\n\n\nline 2",
			expected: "line 1\n\nline 2",
		},
		{
			name:     "compresses repeated lines",
			input:    "Installing dep A\nInstalling dep A\nInstalling dep A\nInstalling dep A\nInstalling dep A\nDone",
			expected: "Installing dep A  [... repeated 4 more times]\nDone",
		},
		{
			name:  "full GitLab CI log example",
			input: "\033[0Ksection_start:1700000000:prepare_executor\r\033[0K\033[36;1mPreparing the executor\033[0m\nUsing Docker executor\nPulling image golang:1.21\n\033[0Ksection_end:1700000000:prepare_executor\r\033[0K\n\033[0Ksection_start:1700000001:build_script\r\033[0K\033[36;1mRunning build\033[0m\n$ go build ./...\nmain.go:15: undefined: foo\n\033[31mERROR: Job failed\033[0m\n\033[0Ksection_end:1700000001:build_script\r\033[0K",
			expected: "## Prepare executor\nPreparing the executor\nUsing Docker executor\nPulling image golang:1.21\n\n## Build script\nRunning build\n$ go build ./...\nmain.go:15: undefined: foo\nERROR: Job failed",
		},
		{
			name:     "section name formatting without header text",
			input:    "section_start:123:my_build_step\nsome output\nsection_end:123:my_build_step",
			expected: "## My build step\nsome output",
		},
		{
			name:     "raw format passthrough",
			input:    "\033[32mHello\033[0m",
			expected: "\033[32mHello\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "raw format passthrough" {
				result := FormatLog(tt.input, LogFormatRaw)
				assert.Equal(t, tt.expected, result)
				return
			}
			result := FormatLogForLLM(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}



func TestFormatLog(t *testing.T) {
	raw := "\033[32mHello\033[0m World"

	assert.Equal(t, raw, FormatLog(raw, LogFormatRaw))
	assert.Equal(t, "Hello World", FormatLog(raw, LogFormatClean))
	assert.Equal(t, "Hello World", FormatLog(raw, LogFormatLLM))
}
