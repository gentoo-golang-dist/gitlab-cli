package git_mock

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/run"
	"gitlab.com/gitlab-org/cli/pkg/utils"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -typed -destination=./git_interface_mock_for_test.go -package=git_mock gitlab.com/gitlab-org/cli/pkg/git_mock GitInterface

const (
	DefaultRemote = "origin"
	DefaultBranch = "master"
)

// ErrNotOnAnyBranch indicates that the user is in detached HEAD state
var ErrNotOnAnyBranch = errors.New("you're not on any Git branch (a 'detached HEAD' state).")

// Ref represents a git commit reference
type Ref struct {
	Hash string
	Name string
}

// Commit represents a git commit
type Commit struct {
	Sha   string
	Title string
}

type (
	GitInterface interface {
		CheckoutBranch(branch string) error
		CheckoutNewBranch(branch string) error
		CurrentBranch() (string, error)
		DeleteLocalBranch(branch string) error
		GetDefaultBranch(remote string) (string, error)
		RemoteBranchExists(remote string, branch string) (bool, error)
		UncommittedChangeCount() (int, error)
		GitUserName() (string, error)
		LatestCommit(ref string) (*Commit, error)
		Commits(baseRef, headRef string) ([]*Commit, error)
		CommitBody(sha string) (string, error)
		Push(remote string, ref string) error
		HasLocalBranch(branch string) (bool, error)
	}

	StandardGitRunner struct {
		gitBinary string
	}
)

var _ GitInterface = (*StandardGitRunner)(nil)

func NewStandardGitRunner(gitBinary string) *StandardGitRunner {
	if gitBinary == "" {
		gitBinary = "git" // default to using "git" from PATH
	}
	return &StandardGitRunner{gitBinary: gitBinary}
}

// CurrentBranch reads the checked-out branch for the git repository
func (g *StandardGitRunner) CurrentBranch() (string, error) {
	stdout, stderr, err := g.runGitCommand("symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return utils.FirstLine(stdout), nil
	}
	var cmdErr *run.CmdError
	if errors.As(err, &cmdErr) {
		if cmdErr.Stderr.Len() == 0 {
			return "", ErrNotOnAnyBranch // Detached HEAD error
		}
	}
	return "", fmt.Errorf("unknown error getting current branch: %v - %s", err, stderr)
}

// GetDefaultBranch gets the default branch for a remote
func (g *StandardGitRunner) GetDefaultBranch(remote string) (string, error) {
	stdout, stderr, err := g.runGitCommand("remote", "show", remote)
	if err != nil {
		return DefaultBranch, fmt.Errorf("could not get default branch for remote %s: %v - %s", remote, err, stderr)
	}

	defaultBranch, err := parseDefaultBranch(stdout)
	if err != nil {
		return DefaultBranch, fmt.Errorf("could not parse default branch for remote %s: %v", remote, err)
	}

	return defaultBranch, nil
}

// DeleteLocalBranch deletes a local git branch
func (g *StandardGitRunner) DeleteLocalBranch(branch string) error {
	_, stderr, err := g.runGitCommand("branch", "-D", branch)
	if err != nil {
		return fmt.Errorf("could not delete local branch %s: %v - %s", branch, err, stderr)
	}
	return nil
}

// CheckoutBranch switches to an existing branch
func (g *StandardGitRunner) CheckoutBranch(branch string) error {
	_, stderr, err := g.runGitCommand("checkout", branch)
	if err != nil {
		return fmt.Errorf("could not checkout branch %s: %v - %s", branch, err, stderr)
	}
	return nil
}

// RemoteBranchExists checks if a remote branch exists
func (g *StandardGitRunner) RemoteBranchExists(remote string, branch string) (bool, error) {
	_, _, err := g.runGitCommand("ls-remote", "--exit-code", "--heads", remote, branch)
	return err == nil, nil // this is either true or false, really
}

// CheckoutNewBranch creates and checks out a new branch
func (g *StandardGitRunner) CheckoutNewBranch(branch string) error {
	_, stderr, err := g.runGitCommand("checkout", "-b", branch)
	if err != nil {
		return fmt.Errorf("could not create new branch: %v - %s", err, stderr)
	}
	return nil
}

// UncommittedChangeCount returns the number of uncommitted changes
func (g *StandardGitRunner) UncommittedChangeCount() (int, error) {
	stdout, stderr, err := g.runGitCommand("status", "--porcelain")
	if err != nil {
		return 0, fmt.Errorf("could not get status: %v - %s", err, stderr)
	}

	lines := strings.Split(string(stdout), "\n")
	count := 0
	for _, l := range lines {
		if l != "" {
			count++
		}
	}
	return count, nil
}

// GitUserName gets the git user name
func (g *StandardGitRunner) GitUserName() (string, error) {
	stdout, stderr, err := g.runGitCommand("config", "user.name")
	if err != nil {
		return "", fmt.Errorf("could not get user name: %v - %s", err, stderr)
	}
	return string(stdout), nil
}

// LatestCommit gets the latest commit for a ref
func (g *StandardGitRunner) LatestCommit(ref string) (*Commit, error) {
	stdout, stderr, err := g.runGitCommand("show", "-s", "--format=%h %s", ref)
	if err != nil {
		return &Commit{}, fmt.Errorf("could not get latest commit: %v - %s", err, stderr)
	}

	split := strings.Fields(string(stdout))

	if len(split) != 2 {
		return &Commit{}, fmt.Errorf("could not parse commit for %s: unexpected format '%s'", ref, string(stdout))
	}

	return &Commit{
		Sha:   split[0],
		Title: split[1],
	}, nil
}

// Commits gets commits between two refs
func (g *StandardGitRunner) Commits(baseRef, headRef string) ([]*Commit, error) {
	stdout, stderr, err := g.runGitCommand(
		"-c", "log.ShowSignature=false",
		"log", "--pretty=format:%H,%s",
		"--cherry", fmt.Sprintf("%s...%s", baseRef, headRef))
	if err != nil {
		return []*Commit{}, fmt.Errorf("could not get commits: %v - %s", err, stderr)
	}

	var commits []*Commit
	for _, line := range outputLines(stdout) {
		split := strings.SplitN(line, ",", 2)
		if len(split) != 2 {
			continue
		}
		commits = append(commits, &Commit{
			Sha:   split[0],
			Title: split[1],
		})
	}

	if len(commits) == 0 {
		return commits, fmt.Errorf("could not find any commits between %s and %s.", baseRef, headRef)
	}

	return commits, nil
}

// CommitBody gets the body of a commit
func (g *StandardGitRunner) CommitBody(sha string) (string, error) {
	stdout, stderr, err := g.runGitCommand("-c", "log.ShowSignature=false", "show", "-s", "--pretty=format:%b", sha)
	if err != nil {
		return "", fmt.Errorf("could not get commit body: %v - %s", err, stderr)
	}
	return string(stdout), nil
}

// Push publishes a git ref to a remote
func (g *StandardGitRunner) Push(remote string, ref string) error {
	_, stderr, err := g.runGitCommand("push", remote, ref)
	if err != nil {
		return fmt.Errorf("could not push: %v - %s", err, stderr)
	}
	return nil
}

// HasLocalBranch checks if a local branch exists
func (g *StandardGitRunner) HasLocalBranch(branch string) (bool, error) {
	_, _, err := g.runGitCommand("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil, nil
}

// runGitCommand executes a git command with proper environment setup and returns stdout/stderr
func (g *StandardGitRunner) runGitCommand(args ...string) ([]byte, string, error) {
	cmd := exec.Command(g.gitBinary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// ensure output from git is in English for string matching
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

// parseDefaultBranch parses the default branch from git remote output
func parseDefaultBranch(output []byte) (string, error) {
	lines := strings.Split(string(output), "\n")

	// try to find "HEAD branch:" line
	headBranchRegex := regexp.MustCompile(`HEAD branch:\s+(.+)`)
	for _, line := range lines {
		if matches := headBranchRegex.FindStringSubmatch(line); len(matches) > 1 {
			return strings.TrimSpace(matches[1]), nil
		}
	}

	// otherwise, look for branch marked with (HEAD)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "(HEAD)") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				return parts[0], nil
			}
		}
	}

	// couldn't find HEAD branch(?)
	return "", errors.New("could not determine default branch from remote output")
}

// outputLines splits output into lines
func outputLines(output []byte) []string {
	lines := strings.TrimSuffix(string(output), "\n")
	return strings.Split(lines, "\n")
}
