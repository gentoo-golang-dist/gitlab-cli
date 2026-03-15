//go:build !integration

package compile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdConfigCompile_hiddenAndMinimal(t *testing.T) {
	t.Parallel()

	cmd := NewCmdConfigCompile(cmdtest.NewTestFactory(nil))
	assert.True(t, cmd.Hidden)
	assert.Empty(t, cmd.Example)
}

func TestPrintDeprecationWarning_warnsOnlyOnTTY(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		isTTY   bool
		wantErr string
	}{
		{
			name:    "without tty",
			isTTY:   false,
			wantErr: "",
		},
		{
			name:    "with tty",
			isTTY:   true,
			wantErr: "Command \"compile\" is deprecated. Use 'glab ci lint --include-merged-yaml' instead.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(tt.isTTY))
			printDeprecationWarning(ios)
			assert.Equal(t, tt.wantErr, stderr.String())
		})
	}
}
