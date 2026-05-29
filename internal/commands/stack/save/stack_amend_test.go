//go:build !integration

package save

import (
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
	}

	for _, tc := range tests {
		description = ""
		t.Run(tc.desc, func(t *testing.T) {
			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
			f := cmdtest.NewTestFactory(ios)

			dir := git.InitGitRepoWithCommit(t)
			err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
			require.Nil(t, err)

			createTemporaryFiles(t, dir, tc.files)

			var saveArgs []string
			saveArgs = append(saveArgs, "-m")
			saveArgs = append(saveArgs, "\"original save message\"")
			saveArgs = append(saveArgs, tc.args...)

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

			createTemporaryFiles(t, dir, tc.amendedFiles)
			if tc.desc == "not on a stack branch" {
				checkout := git.GitCommand("checkout", "-b", "randobranch")
				_, err := run.PrepareCmd(checkout).Output()

				require.Nil(t, err)
			}

			output, err := amendFunc(t.Context(), f, tc.args, getText, tc.description, false, false)

			if tc.wantErr {
				require.ErrorContains(t, err, tc.expected)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expected, output)
			}
		})
	}
}

func Test_stackAmendReword(t *testing.T) {
	t.Run("reword with message flag and clean working tree", func(t *testing.T) {
		description = ""
		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		f := cmdtest.NewTestFactory(ios)

		dir := git.InitGitRepoWithCommit(t)
		err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
		require.Nil(t, err)

		// Create initial stack entry via save
		createTemporaryFiles(t, dir, []string{"initialfile"})

		getText := getMockEditor("", &[]string{})

		ctrl := gomock.NewController(t)
		mockCmd := git_testing.NewMockGitRunner(ctrl)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdSaveStack(f, mockCmd, getText)
		}, true,
			cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
		)
		_, err = exec("-m \"original message\" initialfile")
		require.Nil(t, err)

		// Now reword with NO file changes (clean working tree)
		output, err := amendFunc(t.Context(), f, []string{}, getText, "reworded message", false, true)
		require.Nil(t, err)
		require.Equal(t, "Amended stack item with description: \"reworded message\".\n", output)

		// Verify the stack ref description was updated
		ref, err := git.CurrentStackRefFromCurrentBranch("cool-test-feature")
		require.Nil(t, err)
		require.Equal(t, "reworded message", ref.Description)
	})

	t.Run("reword with --all is rejected", func(t *testing.T) {
		description = ""

		getText := getMockEditor("", &[]string{})

		ctrl := gomock.NewController(t)
		mockCmd := git_testing.NewMockGitRunner(ctrl)

		git.InitGitRepoWithCommit(t)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdAmendStack(f, mockCmd, getText)
		}, true,
			cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
		)
		_, err := exec("--reword --all -m \"some message\"")
		require.ErrorContains(t, err, "if any flags in the group [all reword] are set none of the others can be")
	})

	t.Run("reword with file args is rejected", func(t *testing.T) {
		description = ""
		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))

		dir := git.InitGitRepoWithCommit(t)
		err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
		require.Nil(t, err)

		createTemporaryFiles(t, dir, []string{"initialfile"})

		getText := getMockEditor("", &[]string{})

		ctrl := gomock.NewController(t)
		mockCmd := git_testing.NewMockGitRunner(ctrl)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdSaveStack(f, mockCmd, getText)
		}, true,
			cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
		)
		_, err = exec("-m \"original message\" initialfile")
		require.Nil(t, err)

		// Reword with file args should fail
		_, err = amendFunc(t.Context(), cmdtest.NewTestFactory(ios), []string{"somefile"}, getText, "reworded", false, true)
		require.ErrorContains(t, err, "--reword cannot be used with file arguments")
	})

	t.Run("reword without message opens editor with existing description", func(t *testing.T) {
		description = ""
		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		f := cmdtest.NewTestFactory(ios)

		dir := git.InitGitRepoWithCommit(t)
		err := git.SetLocalConfig("glab.currentstack", "cool-test-feature")
		require.Nil(t, err)

		createTemporaryFiles(t, dir, []string{"initialfile"})

		// Editor returns a new message when invoked
		getText := getMockEditor("editor reworded message", &[]string{})

		ctrl := gomock.NewController(t)
		mockCmd := git_testing.NewMockGitRunner(ctrl)

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdSaveStack(f, mockCmd, getText)
		}, true,
			cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
		)
		_, err = exec("-m \"original message\" initialfile")
		require.Nil(t, err)

		// Reword with no -m flag — should open editor and use its output
		output, err := amendFunc(t.Context(), f, []string{}, getText, "", false, true)
		require.Nil(t, err)
		require.Equal(t, "Amended stack item with description: \"editor reworded message\".\n", output)
	})
}
