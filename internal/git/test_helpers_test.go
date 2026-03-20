//go:build !integration

package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitGitRepo_unsetsHookEnvVars(t *testing.T) {
	// When git runs hooks (e.g., pre-push), it sets GIT_DIR, GIT_WORK_TREE,
	// and GIT_INDEX_FILE. InitGitRepo must unset these so that git commands
	// in temp test repos don't target the parent repository.
	t.Setenv("GIT_DIR", "/some/repo/.git")
	t.Setenv("GIT_WORK_TREE", "/some/repo")
	t.Setenv("GIT_INDEX_FILE", "/some/repo/.git/index")

	InitGitRepo(t)

	_, dirSet := os.LookupEnv("GIT_DIR")
	_, workTreeSet := os.LookupEnv("GIT_WORK_TREE")
	_, indexSet := os.LookupEnv("GIT_INDEX_FILE")

	assert.False(t, dirSet, "GIT_DIR should be unset after InitGitRepo")
	assert.False(t, workTreeSet, "GIT_WORK_TREE should be unset after InitGitRepo")
	assert.False(t, indexSet, "GIT_INDEX_FILE should be unset after InitGitRepo")
}
