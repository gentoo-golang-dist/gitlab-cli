//go:build integration

package checkout

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// Test_MRCheckout_RealGitOutput_Integration runs the checkout command against
// a real `git` subprocess (no mocked GitRunner) so the captured stdout/stderr
// contain the actual bytes git emits for `fetch` and `checkout`. This proves
// the io.Writer plumbing introduced in commit e9bdc3ed (replacing gr.Git with
// gr.GitWithIO) reaches the user end-to-end, not just at the mock boundary.
//
// No GitLab test host is required: the project's clone URL returned by the
// mocked API points at a local bare repo prepared in t.TempDir(), and `git`
// fetches happily from a local path.
func Test_MRCheckout_RealGitOutput_Integration(t *testing.T) {
	originDir := filepath.Join(t.TempDir(), "origin.git")
	seedDir := filepath.Join(t.TempDir(), "seed")
	workDir := filepath.Join(t.TempDir(), "work")

	runGit(t, "", "init", "--bare", "-b", "main", originDir)

	// Seed: create a branch named feat-new-mr with one commit and push it
	// into the bare origin.
	require.NoError(t, os.MkdirAll(seedDir, 0o755))
	runGit(t, seedDir, "init", "-b", "feat-new-mr")
	runGit(t, seedDir, "config", "user.email", "test@example.com")
	runGit(t, seedDir, "config", "user.name", "Integration Test")
	require.NoError(t, os.WriteFile(filepath.Join(seedDir, "hello.txt"), []byte("hi\n"), 0o644))
	runGit(t, seedDir, "add", "hello.txt")
	runGit(t, seedDir, "commit", "-m", "seed commit")
	runGit(t, seedDir, "push", originDir, "feat-new-mr")

	// Work dir: a normal (non-bare) repo with one commit on main. This is
	// where the command will fetch/checkout. cwd must point here for git
	// to operate on the right repo.
	require.NoError(t, os.MkdirAll(workDir, 0o755))
	runGit(t, workDir, "init", "-b", "main")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	runGit(t, workDir, "config", "user.name", "Integration Test")
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "main.txt"), []byte("main\n"), 0o644))
	runGit(t, workDir, "add", "main.txt")
	runGit(t, workDir, "commit", "-m", "main commit")
	t.Chdir(workDir)

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockMergeRequests.EXPECT().
		GetMergeRequest("OWNER/REPO", int64(123), gomock.Any(), gomock.Any()).
		Return(&gitlab.MergeRequest{
			BasicMergeRequest: gitlab.BasicMergeRequest{
				ID:              123,
				IID:             123,
				ProjectID:       3,
				SourceProjectID: 3,
				SourceBranch:    "feat-new-mr",
				State:           "opened",
			},
		}, nil, nil)
	testClient.MockProjects.EXPECT().
		GetProject(gomock.Any(), gomock.Any()).
		Return(&gitlab.Project{
			ID:            3,
			SSHURLToRepo:  originDir,
			HTTPURLToRepo: originDir,
		}, nil, nil)

	ios, _, stdout, stderr := cmdtest.TestIOStreams()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithGitLabClient(testClient.Client))

	cmd := NewCmdCheckout(f)
	argv, err := shlex.Split("123")
	require.NoError(t, err)
	cmd.SetArgs(argv)

	_, err = cmd.ExecuteC()
	require.NoError(t, err, "stdout=%q stderr=%q", stdout.String(), stderr.String())

	// Real `git fetch` writes its summary line (e.g. "* [new branch] ...")
	// to stderr. Real `git checkout` writes "Switched to ..." to stderr too.
	combined := stdout.String() + stderr.String()
	assert.Contains(t, combined, "[new branch]", "fetch output should be captured; stdout=%q stderr=%q", stdout.String(), stderr.String())
	assert.Contains(t, combined, "Switched to", "checkout output should be captured; stdout=%q stderr=%q", stdout.String(), stderr.String())

	// Sanity check: HEAD really points at feat-new-mr — proves the command
	// did more than just print, it actually performed the checkout.
	headRef := strings.TrimSpace(readFile(t, filepath.Join(workDir, ".git", "HEAD")))
	assert.Equal(t, "ref: refs/heads/feat-new-mr", headRef)
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(out))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}
