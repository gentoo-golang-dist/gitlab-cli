//go:build !integration

package infer

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestGenerateStackSha(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	t.Run("produces deterministic output for same inputs", func(t *testing.T) {
		ts := fixedTime()
		sha1, err := generateStackSha("msg", "title", "author", ts)
		require.NoError(t, err)

		sha2, err := generateStackSha("msg", "title", "author", ts)
		require.NoError(t, err)

		assert.Equal(t, sha1, sha2)
	})

	t.Run("produces different output for different inputs", func(t *testing.T) {
		ts := fixedTime()
		sha1, err := generateStackSha("msg1", "title", "author", ts)
		require.NoError(t, err)

		sha2, err := generateStackSha("msg2", "title", "author", ts)
		require.NoError(t, err)

		assert.NotEqual(t, sha1, sha2)
	})

	t.Run("returns 8 hex characters", func(t *testing.T) {
		sha, err := generateStackSha("msg", "title", "author", fixedTime())
		require.NoError(t, err)
		assert.Len(t, sha, 8)
	})
}

func TestCreateShaBranch(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	t.Run("uses configured branch prefix", func(t *testing.T) {
		git.InitGitRepo(t)
		defer config.StubWriteConfig(io.Discard, io.Discard)()

		factory := createFactoryWithConfig("myprefix")

		branch, err := createShaBranch(factory, "abcd1234", "my-stack")
		require.NoError(t, err)
		assert.Equal(t, "myprefix-my-stack-abcd1234", branch)
	})

	t.Run("falls back to USER env var", func(t *testing.T) {
		git.InitGitRepo(t)
		defer config.StubWriteConfig(io.Discard, io.Discard)()

		t.Setenv("USER", "testuser")
		factory := createFactoryWithConfig("")

		branch, err := createShaBranch(factory, "abcd1234", "my-stack")
		require.NoError(t, err)
		assert.Equal(t, "testuser-my-stack-abcd1234", branch)
	})

	t.Run("falls back to glab-stack when no USER", func(t *testing.T) {
		git.InitGitRepo(t)
		defer config.StubWriteConfig(io.Discard, io.Discard)()

		t.Setenv("USER", "")
		factory := createFactoryWithConfig("")

		branch, err := createShaBranch(factory, "abcd1234", "my-stack")
		require.NoError(t, err)
		assert.Equal(t, "glab-stack-my-stack-abcd1234", branch)
	})
}

func TestCreateBranches(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	t.Setenv("USER", "testuser")

	t.Run("creates branches and ref files for each commit", func(t *testing.T) {
		dir := git.InitGitRepoWithCommit(t)
		defer config.StubWriteConfig(io.Discard, io.Discard)()

		stackTitle := "test-stack"
		err := git.SetLocalConfig("glab.currentstack", stackTitle)
		require.NoError(t, err)

		_, err = git.AddStackRefDir(stackTitle)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		mockGR.EXPECT().
			Git("log", "-1", "--format=%s", "abc123").
			Return("First commit", nil)
		mockGR.EXPECT().
			Git("branch", gomock.Any(), "abc123").
			Return("", nil)

		mockGR.EXPECT().
			Git("log", "-1", "--format=%s", "def456").
			Return("Second commit", nil)
		mockGR.EXPECT().
			Git("branch", gomock.Any(), "def456").
			Return("", nil)

		factory := createFactoryWithConfig("test")

		emptyStack := git.Stack{Title: stackTitle, Refs: make(map[string]git.StackRef)}
		commits := []string{"abc123", "def456"}

		err = createBranches(factory, mockGR, commits, stackTitle, emptyStack)
		require.NoError(t, err)

		stack, err := git.GatherStackRefs(stackTitle)
		require.NoError(t, err)
		assert.Len(t, stack.Refs, 2)

		first := stack.First()
		assert.Equal(t, "First commit", first.Description)
		assert.True(t, first.IsFirst())
		assert.False(t, first.IsLast())

		last := stack.Last()
		assert.Equal(t, "Second commit", last.Description)
		assert.False(t, last.IsFirst())
		assert.True(t, last.IsLast())

		assert.Equal(t, first.Next, last.SHA)
		assert.Equal(t, last.Prev, first.SHA)

		_ = dir
	})

	t.Run("appends to existing stack", func(t *testing.T) {
		dir := git.InitGitRepoWithCommit(t)
		defer config.StubWriteConfig(io.Discard, io.Discard)()

		stackTitle := "test-stack"
		err := git.SetLocalConfig("glab.currentstack", stackTitle)
		require.NoError(t, err)

		_, err = git.AddStackRefDir(stackTitle)
		require.NoError(t, err)

		existingRef := git.StackRef{
			SHA:         "existing01",
			Branch:      "test-test-stack-existing01",
			Description: "Existing layer",
		}
		err = git.AddStackRefFile(stackTitle, existingRef)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		mockGR.EXPECT().
			Git("log", "-1", "--format=%s", "newcommit").
			Return("New commit", nil)
		mockGR.EXPECT().
			Git("branch", gomock.Any(), "newcommit").
			Return("", nil)

		factory := createFactoryWithConfig("test")

		existingStack, err := git.GatherStackRefs(stackTitle)
		require.NoError(t, err)
		require.Len(t, existingStack.Refs, 1)

		err = createBranches(factory, mockGR, []string{"newcommit"}, stackTitle, existingStack)
		require.NoError(t, err)

		stack, err := git.GatherStackRefs(stackTitle)
		require.NoError(t, err)
		assert.Len(t, stack.Refs, 2)

		first := stack.First()
		assert.Equal(t, "Existing layer", first.Description)

		last := stack.Last()
		assert.Equal(t, "New commit", last.Description)
		assert.Equal(t, first.SHA, last.Prev)

		_ = dir
	})
}

func TestRunNoTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	t.Run("errors when no stack and no TTY", func(t *testing.T) {
		git.InitGitRepoWithCommit(t)

		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		getText := getMockEditor("", &[]string{})

		exec := cmdtest.SetupCmdForTest(t, func(f cmdutils.Factory) *cobra.Command {
			return NewCmdInferStack(f, mockGR, getText)
		}, false,
			cmdtest.WithGitLabClient(cmdtest.NewTestApiClient(t, nil, "", "gitlab.com").Lab()),
		)

		_, err := exec("main..HEAD")
		require.Error(t, err)
	})
}

func getMockEditor(input string, prompts *[]string) cmdutils.GetTextUsingEditor {
	return func(ctx context.Context, editor, tmpFileName, content string) (string, error) {
		*prompts = append(*prompts, content)
		return input, nil
	}
}

func createFactoryWithConfig(value string) cmdutils.Factory {
	strconfig := heredoc.Doc(`
				branch_prefix: ` + value + `
			`)

	cfg := config.NewFromString(strconfig)
	ios, _, _, _ := cmdtest.TestIOStreams()

	return cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))
}

func fixedTime() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}
