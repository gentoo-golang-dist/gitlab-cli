package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/run"
)

// unsetGitHookEnv removes git hook environment variables (GIT_DIR,
// GIT_WORK_TREE, GIT_INDEX_FILE) that leak when tests run inside a git
// hook (e.g., pre-push via lefthook). Without this, child git processes
// in test temp directories operate against the parent repository.
// The original values are restored when the test completes.
func unsetGitHookEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		if _, ok := os.LookupEnv(key); ok {
			// t.Setenv saves the original value and restores it on cleanup.
			// We then unset the variable so git commands in temp repos don't
			// inherit hook environment from the parent process.
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
	}
}

func InitGitRepo(t *testing.T) string {
	t.Helper()

	unsetGitHookEnv(t)

	tempDir := t.TempDir()

	t.Chdir(tempDir)

	gitInit := GitCommand("init")
	_, err := run.PrepareCmd(gitInit).Output()
	require.NoError(t, err)

	return tempDir
}

func InitGitRepoWithCommit(t *testing.T) string {
	t.Helper()

	tempDir := InitGitRepo(t)

	configureGitConfig(t)

	err := exec.Command("touch", "randomfile").Run()
	require.NoError(t, err)

	gitAdd := GitCommand("add", "randomfile")
	_, err = run.PrepareCmd(gitAdd).Output()
	require.NoError(t, err)

	gitCommit := GitCommand("commit", "-m", "\"commit\"")
	_, err = run.PrepareCmd(gitCommit).Output()
	require.NoError(t, err)

	return tempDir
}

func configureGitConfig(t *testing.T) {
	t.Helper()

	// CI will throw errors using a git command without a configuration
	nameConfig := GitCommand("config", "user.name", "glab test bot")
	_, err := run.PrepareCmd(nameConfig).Output()
	require.NoError(t, err)

	emailConfig := GitCommand("config", "user.email", "no-reply+cli-tests@gitlab.com")
	_, err = run.PrepareCmd(emailConfig).Output()
	require.NoError(t, err)

	gpgConfig := GitCommand("config", "commit.gpgsign", "false")
	_, err = run.PrepareCmd(gpgConfig).Output()
	require.NoError(t, err)
}

// InitGitWorktree creates a git worktree from an existing repo (which must
// have at least one commit) and chdir's into it. Returns the worktree path.
func InitGitWorktree(t *testing.T) string {
	t.Helper()

	// We need a commit before we can create a worktree
	repoDir := InitGitRepoWithCommit(t)

	worktreeDir := t.TempDir()

	addCmd := GitCommand("worktree", "add", worktreeDir, "-b", "worktree-branch")
	addCmd.Dir = repoDir
	_, err := run.PrepareCmd(addCmd).Output()
	require.NoError(t, err)

	t.Chdir(worktreeDir)

	return worktreeDir
}

// InitGitRepoOrWorktree initializes a git repo or worktree based on the
// worktree flag. Returns the directory path.
func InitGitRepoOrWorktree(t *testing.T, worktree bool) string {
	t.Helper()
	if worktree {
		return InitGitWorktree(t)
	}
	return InitGitRepo(t)
}

func CreateRefFiles(refs map[string]StackRef, title string) error {
	for _, ref := range refs {
		err := AddStackRefFile(title, ref)
		if err != nil {
			return err
		}
	}

	return nil
}

func CreateBranches(t *testing.T, branches []string) {
	t.Helper()

	// older versions of git could default to a different branch,
	// so making sure this one exists.
	_ = CheckoutNewBranch("main")

	for _, branch := range branches {
		err := CheckoutNewBranch(branch)
		require.Nil(t, err)
	}
}

// FetchStub returns a closure for MockGitRunner.GitWithIO's DoAndReturn that
// writes a simulated `git fetch` progress line to the stderr writer the
// caller supplies, then returns nil. Use in tests that verify a command
// pipes its stderr writer through to GitWithIO end-to-end.
func FetchStub(refspec string) func(stdout, stderr io.Writer, args ...string) error {
	return func(_, stderr io.Writer, _ ...string) error {
		fmt.Fprintf(stderr, "remote: Counting objects: 100%% (10/10), done.\n* [new branch] %s\n", refspec)
		return nil
	}
}

// FailingFetchStub is the failure variant of FetchStub: writes a stderr line,
// then returns the supplied error message.
func FailingFetchStub(refspec, msg string) func(stdout, stderr io.Writer, args ...string) error {
	return func(_, stderr io.Writer, _ ...string) error {
		fmt.Fprintf(stderr, "fetch attempt %s failed\n", refspec)
		return errors.New(msg)
	}
}

// CheckoutStub returns a closure for MockGitRunner.GitWithIO's DoAndReturn
// that writes a simulated `git checkout` line to the stderr writer the
// caller supplies, then returns nil.
func CheckoutStub(branch string) func(stdout, stderr io.Writer, args ...string) error {
	return func(_, stderr io.Writer, _ ...string) error {
		fmt.Fprintf(stderr, "Switched to a new branch '%s'\n", branch)
		return nil
	}
}

// FailingCheckoutStub is the failure variant of CheckoutStub.
func FailingCheckoutStub(branch, msg string) func(stdout, stderr io.Writer, args ...string) error {
	return func(_, stderr io.Writer, _ ...string) error {
		fmt.Fprintf(stderr, "error: pathspec '%s' did not match\n", branch)
		return errors.New(msg)
	}
}
