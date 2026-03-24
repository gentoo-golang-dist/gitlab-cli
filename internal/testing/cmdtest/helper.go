package cmdtest

import (
	"bytes"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/otiai10/copy"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
)

var (
	ProjectPath    string
	GlabBinaryPath string
)

type fatalLogger interface {
	Fatal(...any)
}

func init() {
	// Unset git hook environment variables that leak when tests run inside
	// a git hook (e.g., pre-push via lefthook). These cause rev-parse and
	// child go-test builds (with -coverpkg=./...) to fail in worktrees.
	// Note: this permanently unsets these vars for the process since init()
	// has no testing.T for cleanup. This is intentional — any test importing
	// cmdtest needs a clean git environment for binary builds.
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		os.Unsetenv(key)
	}

	path := &bytes.Buffer{}
	// get root dir via git
	gitCmd := git.GitCommand("rev-parse", "--show-toplevel")
	gitCmd.Stdout = path
	err := gitCmd.Run()
	if err != nil {
		log.Fatalln("Failed to get root directory: ", err)
	}
	ProjectPath = strings.TrimSuffix(path.String(), "\n")
	if !strings.HasSuffix(ProjectPath, "/") {
		ProjectPath += "/"
	}
}

func InitTest(m *testing.M, suffix string) {
	// Build a glab binary with test symbols. If the parent test binary was run
	// with coverage enabled, enable coverage on the child binary, too.
	var err error
	GlabBinaryPath, err = filepath.Abs(os.ExpandEnv(ProjectPath + "testdata/glab.test"))
	if err != nil {
		log.Fatal(err)
	}
	testCmd := []string{"test", "-c", "-o", GlabBinaryPath, ProjectPath + "cmd/glab"}
	if coverMode := testing.CoverMode(); coverMode != "" {
		testCmd = append(testCmd, "-covermode", coverMode, "-coverpkg", "./...")
	}
	if out, err := exec.Command("go", testCmd...).CombinedOutput(); err != nil {
		log.Fatalf("Error building glab test binary: %s (%s)", string(out), err)
	}

	originalWd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	var repo string = CopyTestRepo(log.New(os.Stderr, "", log.LstdFlags), suffix)

	if err := os.Chdir(repo); err != nil {
		log.Fatalf("Error chdir to test/testdata: %s", err)
	}
	code := m.Run()

	if err := os.Chdir(originalWd); err != nil {
		log.Fatalf("Error chdir to original working dir: %s", err)
	}

	testdirs, err := filepath.Glob(os.ExpandEnv(repo))
	if err != nil {
		log.Printf("Error listing glob test/testdata-*: %s", err)
	}
	for _, dir := range testdirs {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Printf("Error removing dir %s: %s", dir, err)
		}
	}

	os.Exit(code)
}

func CopyTestRepo(log fatalLogger, name string) string {
	if name == "" {
		name = strconv.Itoa(int(rand.Uint64()))
	}
	dest, err := filepath.Abs(os.ExpandEnv(ProjectPath + "test/testdata-" + name))
	if err != nil {
		log.Fatal(err)
	}
	src, err := filepath.Abs(os.ExpandEnv(ProjectPath + "test/testdata"))
	if err != nil {
		log.Fatal(err)
	}
	if err := copy.Copy(src, dest); err != nil {
		log.Fatal(err)
	}
	// Move the test.git dir into the expected path at .git
	if !config.CheckPathExists(dest + "/.git") {
		if err := os.Rename(dest+"/test.git", dest+"/.git"); err != nil {
			log.Fatal(err)
		}
	}
	// Move the test.glab-cli dir into the expected path at .glab-cli
	if !config.CheckPathExists(dest + "/.glab-cli") {
		if err := os.Rename(dest+"/test.glab-cli", dest+"/.glab-cli"); err != nil {
			log.Fatal(err)
		}
	}
	return dest
}
