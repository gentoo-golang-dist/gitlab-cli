//go:build !integration

package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

var tests = []struct {
	name        string
	args        string
	expectedOut string
	expectedErr string
}{
	{
		name:        "when no args should display the help message",
		args:        "",
		expectedOut: "Use \"ci [command] --help\" for more information about a command.\n",
		expectedErr: "Aliases 'pipe' and 'pipeline' are deprecated. Use 'ci' instead.",
	},
}

func TestPipelineCmd(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantedErr := ""
			if len(test.expectedErr) > 0 {
				wantedErr = test.expectedErr
			}

			ios, _, stdout, stderr := cmdtest.TestIOStreams()
			f := cmdtest.NewTestFactory(ios)

			cmd := NewCmdCI(f)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)

			err := cmd.Execute()

			if assert.NoErrorf(t, err, "error running `ci %s`: %v", test.args, err) {
				assert.Contains(t, stderr.String(), wantedErr)
				assert.Contains(t, stdout.String(), test.expectedOut)
			}
		})
	}
}
