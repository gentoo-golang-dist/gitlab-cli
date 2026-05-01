//go:build !integration

package lint

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func testdataPath(parts ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(filename), "testdata")
	return filepath.Join(append([]string{base}, parts...)...)
}

func newLintOptions(t *testing.T) *options {
	t.Helper()

	ios, _, _, _ := cmdtest.TestIOStreams()
	repo := glrepo.New("OWNER", "REPO", glinstance.DefaultHostname)
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockProjects.EXPECT().
		GetProject("OWNER/REPO", gomock.Any()).
		Return(&gitlab.Project{ID: 123}, nil, nil)

	return &options{
		path:        testdataPath(".gitlab-ci.yaml"),
		defaultPath: ".gitlab-ci.yml",
		io:          ios,
		repoFactory: func() (glrepo.Interface, error) { return repo, nil },
		clientFactory: func() (*gitlab.Client, error) {
			return testClient.Client, nil
		},
	}
}

func stubGitRefs(
	t *testing.T,
	describe func() (string, error),
	branch func() (string, error),
	defaultRef func(glrepo.Interface, *gitlab.Client) string,
) {
	t.Helper()

	oldDescribe, oldBranch, oldDefault := describeByTags, currentBranch, resolveDefaultRef
	if describe != nil {
		describeByTags = describe
	}
	if branch != nil {
		currentBranch = branch
	}
	if defaultRef != nil {
		resolveDefaultRef = defaultRef
	}
	t.Cleanup(func() {
		describeByTags, currentBranch, resolveDefaultRef = oldDescribe, oldBranch, oldDefault
	})
}

func newTestIO(stdout io.Writer, stdoutTTY bool, stderr io.Writer, stderrTTY bool) *iostreams.IOStreams {
	return iostreams.New(
		iostreams.WithStdin(io.NopCloser(bytes.NewBuffer(nil)), false),
		iostreams.WithStdout(stdout, stdoutTTY),
		iostreams.WithStderr(stderr, stderrTTY),
	)
}
