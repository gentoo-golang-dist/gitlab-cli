//go:build !integration

package sync

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/run"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func gitRun(t *testing.T, args ...string) string {
	t.Helper()
	cmd := git.GitCommand(args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	out, err := run.PrepareCmd(cmd).Output()
	require.NoError(t, err, "git %s failed: %s", strings.Join(args, " "), string(out))
	return strings.TrimSpace(string(out))
}

func gitRunMayFail(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := git.GitCommand(args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	out, err := run.PrepareCmd(cmd).Output()
	return strings.TrimSpace(string(out)), err
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
}

func readFileContent(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(data)
}

func setupStackedRepo(t *testing.T) git.Stack {
	t.Helper()
	git.InitGitRepoWithCommit(t)

	gitRun(t, "branch", "-M", "main")

	writeFile(t, "base.txt", "base content\n")
	gitRun(t, "add", "base.txt")
	gitRun(t, "commit", "-m", "base commit")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "file1.txt", "content from branch1\n")
	gitRun(t, "add", "file1.txt")
	gitRun(t, "commit", "-m", "branch1: add file1")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "file2.txt", "content from branch2\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "-m", "branch2: add file2")

	title := "test-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1: add file1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "",
		Branch: "stack-branch2", Description: "branch2: add file2",
		Base: branch1SHA,
	}

	err := git.AddStackRefFile(title, ref1)
	require.NoError(t, err)
	err = git.AddStackRefFile(title, ref2)
	require.NoError(t, err)

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	return stack
}

func setupStackedRepoThreeEntries(t *testing.T) git.Stack {
	t.Helper()
	git.InitGitRepoWithCommit(t)

	gitRun(t, "branch", "-M", "main")

	writeFile(t, "base.txt", "base content\n")
	gitRun(t, "add", "base.txt")
	gitRun(t, "commit", "-m", "base commit")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "file1.txt", "content from branch1\n")
	gitRun(t, "add", "file1.txt")
	gitRun(t, "commit", "-m", "branch1: add file1")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "file2.txt", "content from branch2\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "-m", "branch2: add file2")

	branch2SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch3")
	writeFile(t, "file3.txt", "content from branch3\n")
	gitRun(t, "add", "file3.txt")
	gitRun(t, "commit", "-m", "branch3: add file3")

	title := "test-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1: add file1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "ref3",
		Branch: "stack-branch2", Description: "branch2: add file2",
		Base: branch1SHA,
	}
	ref3 := git.StackRef{
		SHA: "ref3", Prev: "ref2", Next: "",
		Branch: "stack-branch3", Description: "branch3: add file3",
		Base: branch2SHA,
	}

	err := git.AddStackRefFile(title, ref1)
	require.NoError(t, err)
	err = git.AddStackRefFile(title, ref2)
	require.NoError(t, err)
	err = git.AddStackRefFile(title, ref3)
	require.NoError(t, err)

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	return stack
}

func Test_rebaseOnto_avoidsConflictWhenBranch1Amended(t *testing.T) {
	stack := setupStackedRepo(t)
	var gr git.StandardGitCommand
	ios, _, _, _ := cmdtest.TestIOStreams()

	oldBranch1SHA := gitRun(t, "rev-parse", "stack-branch1")

	gitRun(t, "checkout", "stack-branch1")
	writeFile(t, "file1.txt", "amended content from branch1\n")
	gitRun(t, "add", "file1.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: add file1 (amended)")

	ref2 := stack.Refs["ref2"]
	assert.Equal(t, oldBranch1SHA, ref2.Base)

	err := rebaseOntoWithUpdateRefs(ios, "stack-branch1", ref2.Base, &stack, gr)
	assert.NoError(t, err, "rebaseOntoWithUpdateRefs should succeed (skips old branch1 commit)")

	gitRun(t, "checkout", "stack-branch2")
	content := readFileContent(t, "file1.txt")
	assert.Equal(t, "amended content from branch1\n", content,
		"branch2 should have the amended file1 content")

	assert.FileExists(t, "file2.txt")
	content2 := readFileContent(t, "file2.txt")
	assert.Equal(t, "content from branch2\n", content2,
		"branch2 should still have its own file2")
}

func Test_updateBase_newRebase_nonConflictingAdvance(t *testing.T) {
	git.InitGitRepoWithCommit(t)
	var gr git.StandardGitCommand

	gitRun(t, "branch", "-M", "main")

	writeFile(t, "base.txt", "original base\n")
	gitRun(t, "add", "base.txt")
	gitRun(t, "commit", "-m", "base: add base.txt")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "file1.txt", "branch1 work\n")
	gitRun(t, "add", "file1.txt")
	gitRun(t, "commit", "-m", "branch1: add file1")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "file2.txt", "branch2 work\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "-m", "branch2: add file2")

	gitRun(t, "checkout", "main")
	writeFile(t, "mainfile.txt", "main advanced\n")
	gitRun(t, "add", "mainfile.txt")
	gitRun(t, "commit", "-m", "main: add mainfile")

	title := "updatebase-advance-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "",
		Branch: "stack-branch2", Description: "branch2",
		Base: branch1SHA,
	}
	require.NoError(t, git.AddStackRefFile(title, ref1))
	require.NoError(t, git.AddStackRefFile(title, ref2))

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()

	err = rebaseWithUpdateRefs(ios, "main", &stack, gr)
	assert.NoError(t, err, "rebase with --onto should succeed when base advances with non-conflicting changes")

	gitRun(t, "checkout", "stack-branch2")
	assert.FileExists(t, "mainfile.txt", "branch2 should have new file from main")
	assert.FileExists(t, "file1.txt", "branch2 should still have file1")
	assert.FileExists(t, "file2.txt", "branch2 should still have file2")
}

func Test_rebaseOnto_threeEntries_middleBranchAmended(t *testing.T) {
	stack := setupStackedRepoThreeEntries(t)
	var gr git.StandardGitCommand
	ios, _, _, _ := cmdtest.TestIOStreams()

	oldBranch2SHA := gitRun(t, "rev-parse", "stack-branch2")

	gitRun(t, "checkout", "stack-branch2")
	writeFile(t, "file2.txt", "amended content from branch2\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "--amend", "-m", "branch2: add file2 (amended)")

	ref3 := stack.Refs["ref3"]
	assert.Equal(t, oldBranch2SHA, ref3.Base)

	err := rebaseOntoWithUpdateRefs(ios, "stack-branch2", ref3.Base, &stack, gr)
	assert.NoError(t, err, "rebaseOntoWithUpdateRefs should succeed (skips old branch2 commit)")

	gitRun(t, "checkout", "stack-branch3")
	content := readFileContent(t, "file2.txt")
	assert.Equal(t, "amended content from branch2\n", content,
		"branch3 should have the amended file2 content")

	assert.FileExists(t, "file3.txt")
	content3 := readFileContent(t, "file3.txt")
	assert.Equal(t, "content from branch3\n", content3,
		"branch3 should still have its own file3")
}

func Test_rebaseOnto_divergedBranch_amendedSameFile_succeeds(t *testing.T) {
	git.InitGitRepoWithCommit(t)
	var gr git.StandardGitCommand

	gitRun(t, "branch", "-M", "main")

	writeFile(t, "shared.txt", "original\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "base")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "shared.txt", "modified by branch1\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "branch1: modify shared")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "file2.txt", "branch2 work\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "-m", "branch2: add file2")

	branch2SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch3")
	writeFile(t, "file3.txt", "branch3 work\n")
	gitRun(t, "add", "file3.txt")
	gitRun(t, "commit", "-m", "branch3: add file3")

	title := "diverged-amend-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "ref3",
		Branch: "stack-branch2", Description: "branch2",
		Base: branch1SHA,
	}
	ref3 := git.StackRef{
		SHA: "ref3", Prev: "ref2", Next: "",
		Branch: "stack-branch3", Description: "branch3",
		Base: branch2SHA,
	}
	require.NoError(t, git.AddStackRefFile(title, ref1))
	require.NoError(t, git.AddStackRefFile(title, ref2))
	require.NoError(t, git.AddStackRefFile(title, ref3))

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()

	gitRun(t, "checkout", "stack-branch1")
	writeFile(t, "shared.txt", "suggestion applied on branch1\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: apply suggestion to shared")

	err = rebaseOntoWithUpdateRefs(ios, "stack-branch1", ref2.Base, &stack, gr)
	assert.NoError(t, err, "rebaseOnto should succeed: skips old branch1 commit, avoids conflict on shared.txt")

	gitRun(t, "checkout", "stack-branch3")
	content := readFileContent(t, "shared.txt")
	assert.Equal(t, "suggestion applied on branch1\n", content,
		"branch3 should have the suggestion-applied version of shared.txt")
	assert.FileExists(t, "file2.txt")
	assert.FileExists(t, "file3.txt")
}

func Test_rebaseOnto_realConflict_sameFile_bothFail(t *testing.T) {
	git.InitGitRepoWithCommit(t)
	var gr git.StandardGitCommand

	gitRun(t, "branch", "-M", "main")

	writeFile(t, "shared.txt", "original\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "base")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "shared.txt", "modified by branch1\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "branch1: modify shared")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "shared.txt", "modified differently by branch2\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "branch2: also modify shared")

	title := "real-conflict-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "",
		Branch: "stack-branch2", Description: "branch2",
		Base: branch1SHA,
	}
	require.NoError(t, git.AddStackRefFile(title, ref1))
	require.NoError(t, git.AddStackRefFile(title, ref2))

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()

	gitRun(t, "checkout", "stack-branch1")
	writeFile(t, "shared.txt", "suggestion applied on branch1\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: apply suggestion")

	err = rebaseOntoWithUpdateRefs(ios, "stack-branch1", ref2.Base, &stack, gr)
	assert.Error(t, err, "new rebase should fail: branch2 independently modifies shared.txt, genuine conflict")

	_, _ = gitRunMayFail(t, "rebase", "--abort")
}

func Test_rebaseOnto_updateBase_avoidsConflictAfterBaseAdvances(t *testing.T) {
	stack := setupStackedRepo(t)
	var gr git.StandardGitCommand
	ios, _, _, _ := cmdtest.TestIOStreams()

	gitRun(t, "checkout", "main")
	writeFile(t, "newfile.txt", "new content on main\n")
	gitRun(t, "add", "newfile.txt")
	gitRun(t, "commit", "-m", "main: add newfile")

	err := rebaseWithUpdateRefs(ios, "main", &stack, gr)
	assert.NoError(t, err, "rebase with --onto should succeed when base branch advances")

	gitRun(t, "checkout", "stack-branch2")
	assert.FileExists(t, "newfile.txt", "branch2 should have the new file from main")
	assert.FileExists(t, "file1.txt", "branch2 should still have file1")
	assert.FileExists(t, "file2.txt", "branch2 should still have file2")

	gitRun(t, "checkout", "stack-branch1")
	assert.FileExists(t, "newfile.txt", "branch1 should have the new file from main")
	assert.FileExists(t, "file1.txt", "branch1 should still have file1")
}

func Test_rebaseOnto_realConflict_bothApproachesFail(t *testing.T) {
	stack := setupStackedRepo(t)
	var gr git.StandardGitCommand
	ios, _, _, _ := cmdtest.TestIOStreams()

	oldBranch1SHA := gitRun(t, "rev-parse", "stack-branch1")

	gitRun(t, "checkout", "stack-branch1")
	writeFile(t, "file2.txt", "conflicting content written in branch1 amend\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: amended with conflict on file2")

	ref2 := stack.Refs["ref2"]

	err := rebaseOntoWithUpdateRefs(ios, "stack-branch1", ref2.Base, &stack, gr)
	assert.Error(t, err, "rebaseOntoWithUpdateRefs should fail when there's a real conflict on file2")

	gitRun(t, "rebase", "--abort")

	gitRun(t, "checkout", "stack-branch1")
	gitRun(t, "reset", "--hard", oldBranch1SHA)
	writeFile(t, "file2.txt", "conflicting content written in branch1 amend\n")
	gitRun(t, "add", "file2.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: amended with conflict on file2")

	stackNoBase := stack
	for sha, ref := range stackNoBase.Refs {
		ref.Base = ""
		stackNoBase.Refs[sha] = ref
	}

	err = rebaseWithUpdateRefs(ios, "stack-branch1", &stackNoBase, gr)
	assert.Error(t, err, "old rebase should also fail when there's a real conflict on file2")

	gitRun(t, "rebase", "--abort")
}

func Test_rebaseOnto_sameLine_conflict_bothFail(t *testing.T) {
	git.InitGitRepoWithCommit(t)

	gitRun(t, "branch", "-M", "main")
	writeFile(t, "shared.txt", "line1\nline2\nline3\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "base: add shared.txt")

	baseSHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch1")
	writeFile(t, "shared.txt", "line1\nmodified-by-branch1\nline3\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "branch1: modify line2")

	branch1SHA := gitRun(t, "rev-parse", "HEAD")

	gitRun(t, "checkout", "-b", "stack-branch2")
	writeFile(t, "shared.txt", "line1\nmodified-by-branch2\nline3\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "-m", "branch2: also modify line2")

	title := "conflict-stack"
	ref1 := git.StackRef{
		SHA: "ref1", Prev: "", Next: "ref2",
		Branch: "stack-branch1", Description: "branch1",
		Base: baseSHA,
	}
	ref2 := git.StackRef{
		SHA: "ref2", Prev: "ref1", Next: "",
		Branch: "stack-branch2", Description: "branch2",
		Base: branch1SHA,
	}
	require.NoError(t, git.AddStackRefFile(title, ref1))
	require.NoError(t, git.AddStackRefFile(title, ref2))

	stack, err := git.GatherStackRefs(title)
	require.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()
	var gr git.StandardGitCommand

	gitRun(t, "checkout", "stack-branch1")
	writeFile(t, "shared.txt", "line1\namended-by-branch1\nline3\n")
	gitRun(t, "add", "shared.txt")
	gitRun(t, "commit", "--amend", "-m", "branch1: amend line2")

	err = rebaseOntoWithUpdateRefs(ios, "stack-branch1", ref2.Base, &stack, gr)
	assert.Error(t, err, "new rebase should fail: branch2 modifies same line as amended branch1")

	gitRun(t, "rebase", "--abort")

	stackNoBase := stack
	for sha, ref := range stackNoBase.Refs {
		ref.Base = ""
		stackNoBase.Refs[sha] = ref
	}

	err = rebaseWithUpdateRefs(ios, "stack-branch1", &stackNoBase, gr)
	assert.Error(t, err, "old rebase should also fail: same conflict")

	gitRun(t, "rebase", "--abort")
}

func Test_refreshBaseRefs_updatesAllBases(t *testing.T) {
	stack := setupStackedRepo(t)
	var gr git.StandardGitCommand

	err := git.AddStackBaseBranch(stack.Title, "main")
	require.NoError(t, err)

	err = refreshBaseRefs(&stack, gr)
	require.NoError(t, err)

	mainSHA := gitRun(t, "rev-parse", "main")
	branch1SHA := gitRun(t, "rev-parse", "stack-branch1")

	ref1 := stack.Refs["ref1"]
	ref2 := stack.Refs["ref2"]

	assert.Equal(t, mainSHA, ref1.Base, "ref1.Base should be main's SHA")
	assert.Equal(t, branch1SHA, ref2.Base, "ref2.Base should be branch1's SHA")
}
