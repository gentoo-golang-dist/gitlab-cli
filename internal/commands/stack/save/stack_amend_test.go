//go:build !integration

package save

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/run"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_stackAmendCmd(t *testing.T) {
	tests := []struct {
		desc          string
		args          []string
		files         []string
		amendedFiles  []string
		modifyFiles   []string // Files to modify (not create new) for testing -u behavior
		description   string
		expected      string
		wantErr       bool
		editorMessage string
	}{
		{
			desc:         "amending regular files",
			args:         []string{"testfile", "randomfile"},
			files:        []string{"testfile", "randomfile"},
			amendedFiles: []string{"otherfile"},
			description:  "this is a commit message",
			expected:     "Amended stack item with description: \"this is a commit message\".\n",
		},
		{
			desc:          "with no message",
			args:          []string{"testfile", "randomfile"},
			files:         []string{"testfile", "randomfile"},
			amendedFiles:  []string{"otherfile"},
			description:   "",
			editorMessage: "amended description",
			expected:      "Amended stack item with description: \"amended description\".\n",
		},
		{
			desc:         "with no amended changes",
			args:         []string{"."},
			files:        []string{"oldfile"},
			amendedFiles: []string{},
			description:  "this is a commit message",
			expected:     "no changes to save",
			wantErr:      true,
		},
		{
			desc:         "not on a stack branch",
			args:         []string{"asdf"},
			files:        []string{"asdf"},
			amendedFiles: []string{"otherfile"},
			description:  "this is a commit message",
			expected:     "Could not find stack ref for branch",
			wantErr:      true,
		},
		{
			desc:         "amend without arguments should not add untracked files",
			args:         []string{},
			files:        []string{"trackedfile"},
			modifyFiles:  []string{"trackedfile"},
			amendedFiles: []string{"untrackedfile"},
			description:  "amend only tracked changes",
			expected:     "Amended stack item with description: \"amend only tracked changes\".\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
			f := cmdtest.NewTestFactory(ios)

			dir := git.InitGitRepoWithCommit(t)
			err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
			require.Nil(t, err)

			createTemporaryFiles(t, dir, tc.files)

			// Use tc.files or tc.args for save args, but only if tc.files is not empty
			var saveArgs []string
			saveArgs = append(saveArgs, "-m")
			saveArgs = append(saveArgs, "\"original save message\"")
			if len(tc.files) > 0 {
				saveArgs = append(saveArgs, tc.files...)
			}

			getText := getMockEditor(tc.editorMessage, &[]string{})

			ctrl := gomock.NewController(t)
			mockCmd := git_testing.NewMockGitRunner(ctrl)

			exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
				return NewCmdSaveStack(f, mockCmd, getText)
			}, true,
				cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
			)
			_, err = exec(strings.Join(saveArgs, " "))
			require.Nil(t, err)

			// Modify tracked files if specified (for testing git add -u behavior)
			for _, file := range tc.modifyFiles {
				filePath := path.Join(dir, file)
				err := os.WriteFile(filePath, []byte("modified content"), 0644)
				require.Nil(t, err)
			}

			// Create new untracked files
			createTemporaryFiles(t, dir, tc.amendedFiles)
			if tc.desc == "not on a stack branch" {
				checkout := git.GitCommand("checkout", "-b", "randobranch")
				_, err := run.PrepareCmd(checkout).Output()

				require.Nil(t, err)
			}

			output, err := amendFunc(t.Context(), f, tc.args, getText, tc.description)

			if tc.wantErr {
				require.ErrorContains(t, err, tc.expected)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expected, output)
			}
		})
	}
}
