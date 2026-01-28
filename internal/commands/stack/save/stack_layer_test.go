//go:build !integration

package save

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestLayerNewStack(t *testing.T) {
	// NOTE: we need to force disable colors, otherwise we'd need ANSI sequences in our test output assertions.
	t.Setenv("NO_COLOR", "true")

	tests := []struct {
		desc          string
		args          []string
		files         []string
		message       string
		expected      string
		wantErr       bool
		noTTY         bool
		editorMessage string
	}{
		{
			desc:     "adding regular files",
			args:     []string{"testfile", "randomfile"},
			files:    []string{"testfile", "randomfile"},
			message:  "this is a commit message",
			expected: "• cool-test-feature: Created new layer with message: \"this is a commit message\".\n",
		},

		{
			desc:     "adding files with a dot argument",
			args:     []string{"."},
			files:    []string{"testfile", "randomfile"},
			message:  "this is a commit message",
			expected: "• cool-test-feature: Created new layer with message: \"this is a commit message\".\n",
		},

		{
			desc:     "adding files with no argument",
			args:     []string{""},
			files:    []string{"testfile", "randomfile"},
			message:  "this is a commit message",
			expected: "• cool-test-feature: Created new layer with message: \"this is a commit message\".\n",
		},

		{
			desc:          "omitting a message",
			args:          []string{"."},
			files:         []string{"testfile"},
			editorMessage: "oh ok fine how about blah blah",
			expected:      "• cool-test-feature: Created new layer with message: \"oh ok fine how about blah blah\".\n",
		},

		{
			desc:     "with no changed files",
			args:     []string{"."},
			files:    []string{},
			expected: "could not create layer: \"no changes to save.\"",
			wantErr:  true,
		},

		{
			desc:     "Test with no message and noTTY",
			args:     []string{"."},
			files:    []string{"testfile"},
			expected: "glab stack layer without `-m` and without a TTY should throw an error.",
			wantErr:  true,
			noTTY:    true,
		},
	}

	for _, tc := range tests {
		isTTY := !tc.noTTY
		t.Run(tc.desc, func(t *testing.T) {
			if tc.message != "" && isTTY {
				tc.args = append(tc.args, "-m")
				tc.args = append(tc.args, "\""+tc.message+"\"")
			}

			dir := git.InitGitRepoWithCommit(t)
			err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
			require.Nil(t, err)

			createTemporaryLayerFiles(t, dir, tc.files)

			getText := getMockEditor(tc.editorMessage, &[]string{})
			args := strings.Join(tc.args, " ")

			ctrl := gomock.NewController(t)
			mockCmd := git_testing.NewMockGitRunner(ctrl)

			exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
				return NewCmdLayerStack(f, mockCmd, getText)
			}, isTTY,
				cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
			)

			output, err := exec(args)

			if tc.wantErr {
				require.Errorf(t, err, tc.expected)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expected, output.String())
			}
		})
	}
}

func Test_generateLayerSha(t *testing.T) {
	type args struct {
		message   string
		title     string
		author    string
		timestamp time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic test",
			args: args{message: "hello", title: "supercool stack title", author: "norm maclean", timestamp: time.Date(1998, time.July, 6, 1, 3, 3, 7, time.UTC)},
			want: "e062296a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git.InitGitRepo(t)

			got, err := generateLayerSha(tt.args.message, tt.args.title, tt.args.author, tt.args.timestamp)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				require.Equal(t, got, tt.want)
			}
		})
	}
}

func createTemporaryLayerFiles(t *testing.T, dir string, files []string) {
	t.Helper()

	for _, file := range files {
		file = path.Join(dir, file)
		_, err := os.Create(file)

		require.Nil(t, err)
	}
}
