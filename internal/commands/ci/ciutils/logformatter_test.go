//go:build !integration

package ciutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatLogCompact(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLogCompact(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLogMinimal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "collapses boilerplate sections",
			input:    "section_start:1:prepare_executor\nUsing Docker executor\nPulling image\nsection_end:1:prepare_executor\nsection_start:2:build_script\n$ go build ./...\nERROR: build failed\nsection_end:2:build_script",
			expected: "[Prepare executor: ok]\n\n## Build script\n$ go build ./...\nERROR: build failed",
		},
		{
			name:     "strips runner metadata before first section",
			input:    "Running with gitlab-runner 18.4.0 (abc123)\non runner-abc jp7oyWQbz, system ID: r_xyz\nfeature flags: FF_USE_FASTZIP:true\nResolving secrets\nsection_start:1:build_script\n$ make test\nsection_end:1:build_script",
			expected: "## Build script\n$ make test",
		},
		{
			name:     "removes git remote banners",
			input:    "section_start:1:build_script\nremote:\nremote: ========\nremote: Maintenance window\nremote: ========\nremote:\nactual output\nsection_end:1:build_script",
			expected: "## Build script\nactual output",
		},
		{
			name:     "removes curl transfer stats",
			input:    "section_start:1:build_script\n% Total    % Received\nDload  Upload   Total\n  100  1234    0  1234\nResponse: ok\nsection_end:1:build_script",
			expected: "## Build script\nResponse: ok",
		},
		{
			name:     "removes shell setup commands",
			input:    "section_start:1:build_script\n$ export FOO=bar\n$ chmod +x script.sh\n$ cat > file.txt <<'EOF'\n$ printf \"hello\" > out.txt\n$ source /venv/bin/activate\nactual work here\nsection_end:1:build_script",
			expected: "## Build script\nactual work here",
		},
		{
			name:     "removes secret mapping block",
			input:    "section_start:1:build_script\n[INFO] SECRETS mapping:\n[INFO]   \"MY_SECRET:MY_SECRET\"\n[INFO]   \"OTHER_KEY:OTHER_KEY\"\nnext real line\nsection_end:1:build_script",
			expected: "## Build script\nnext real line",
		},
		{
			name:     "removes decorative separator lines",
			input:    "section_start:1:build_script\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\nImportant info\n──────────────────────────────\nsection_end:1:build_script",
			expected: "## Build script\nImportant info",
		},
		{
			name:     "removes executing stage boilerplate",
			input:    "section_start:1:step_script\nExecuting \"step_script\" stage of the job script\n$ make build\nDone\nsection_end:1:step_script",
			expected: "## Step script\n$ make build\nDone",
		},
		{
			name:     "collapses after_script section",
			input:    "section_start:1:step_script\n$ make build\nDone\nsection_end:1:step_script\nsection_start:2:after_script\nRunning after script\n$ cleanup.sh\nsection_end:2:after_script",
			expected: "## Step script\n$ make build\nDone\n[After script: ok]",
		},
		{
			name:     "preserves non-boilerplate sections fully",
			input:    "section_start:1:prepare_executor\nsetup stuff\nsection_end:1:prepare_executor\nsection_start:2:build_script\n$ go test ./...\nFAIL main_test.go:15\nsection_end:2:build_script",
			expected: "[Prepare executor: ok]\n\n## Build script\n$ go test ./...\nFAIL main_test.go:15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLogMinimal(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLog(t *testing.T) {
	raw := "\033[32mHello\033[0m World"

	assert.Equal(t, raw, FormatLog(raw, LogFormatRaw))
	assert.Equal(t, "Hello World", FormatLog(raw, LogFormatClean))
	assert.Equal(t, "Hello World", FormatLog(raw, LogFormatMinimal))
	assert.Equal(t, "Hello World", FormatLog(raw, LogFormatCompact))
}
