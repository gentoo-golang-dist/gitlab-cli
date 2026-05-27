//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -typed -destination=./testing/git_runner.go -package=git gitlab.com/gitlab-org/cli/internal/git GitRunner

package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/run"
)

// StackLocation returns the path to the stacked metadata directory.
// It uses git rev-parse --git-dir so it works in worktrees.
func StackLocation() (string, error) {
	gitDir, err := GitDir()
	if err != nil {
		return "", fmt.Errorf("finding git directory: %w", err)
	}
	return filepath.Join(gitDir, "stacked"), nil
}

const BaseBranchFile = "BASE_BRANCH"

type GitRunner interface {
	Git(args ...string) (string, error)
}

// Executor is a subset of cmdutils.Executor defined here to avoid a circular
// import: the cmdutils package already imports the git package.
type Executor interface {
	ExecWithIO(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type StandardGitCommand struct {
	executor Executor
}

func NewGitCommand(executor Executor) StandardGitCommand {
	return StandardGitCommand{executor: executor}
}

func (gitc StandardGitCommand) Git(args ...string) (string, error) {
	env := append(os.Environ(), "LC_ALL=C")

	if gitc.executor == nil {
		cmd := GitCommand(args...)
		cmd.Env = env
		output, err := run.PrepareCmd(cmd).Output()
		if err != nil {
			return "", err
		}
		return string(output), nil
	}

	var stdout, stderr bytes.Buffer
	err := gitc.executor.ExecWithIO(context.Background(), "git", args, env, nil, &stdout, &stderr)
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%s: %w", errMsg, err)
		}
		return "", err
	}
	return stdout.String(), nil
}

func SetLocalConfig(key, value string) error {
	found, err := configValueExists(key, value)
	if err != nil {
		return fmt.Errorf("Git config value exists: %w", err)
	}

	if found {
		return nil
	}

	addCmd := GitCommand("config", "--local", key, value)
	_, err = run.PrepareCmd(addCmd).Output()
	if err != nil {
		return fmt.Errorf("setting local Git config: %w", err)
	}
	return nil
}

func GetCurrentStackTitle() (string, error) {
	return Config("glab.currentstack")
}

func AddStackRefDir(dir string) (string, error) {
	stackLoc, err := StackLocation()
	if err != nil {
		return "", fmt.Errorf("finding stack location: %w", err)
	}

	createdDir := filepath.Join(stackLoc, dir)

	err = os.MkdirAll(createdDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating stacked diff directory: %w", err)
	}

	return createdDir, nil
}

func StackRootDir(title string) (string, error) {
	stackLoc, err := StackLocation()
	if err != nil {
		return "", err
	}

	return filepath.Join(stackLoc, title), nil
}

func AddStackRefFile(title string, stackRef StackRef) error {
	refDir, err := StackRootDir(title)
	if err != nil {
		return fmt.Errorf("error determining Git root: %v", err)
	}

	initialJsonData, err := json.Marshal(stackRef) //nolint:forbidigo // stack reference is written to disk, not stdout
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	if _, err = os.Stat(refDir); os.IsNotExist(err) {
		err = os.MkdirAll(refDir, 0o700) // create directory if it doesn't exist
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
	}

	fullPath := filepath.Join(refDir, stackRef.SHA+".json")

	err = os.WriteFile(fullPath, initialJsonData, 0o644)
	if err != nil {
		return fmt.Errorf("error running writing file: %v", err)
	}

	return nil
}

func DeleteStackRefFile(title string, stackRef StackRef) error {
	refDir, err := StackRootDir(title)
	if err != nil {
		return fmt.Errorf("error determining Git root: %v", err)
	}

	fullPath := filepath.Join(refDir, stackRef.SHA+".json")

	err = os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("error removing stack file: %v", err)
	}

	return nil
}

func UpdateStackRefFile(title string, s StackRef) error {
	refDir, err := StackRootDir(title)
	if err != nil {
		return fmt.Errorf("error determining Git root: %v", err)
	}

	fullPath := filepath.Join(refDir, s.SHA+".json")

	initialJsonData, err := json.Marshal(s) //nolint:forbidigo // stack ref file is written to disk, not stdout
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	err = os.WriteFile(fullPath, initialJsonData, 0o644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	return nil
}

func GetStacks() ([]Stack, error) {
	stackLocationDir, err := StackLocation()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(stackLocationDir)
	if err != nil {
		return nil, err
	}
	var stacks []Stack
	for _, v := range entries {
		if !v.IsDir() {
			continue
		}
		stacks = append(stacks, Stack{Title: v.Name()})
	}
	return stacks, nil
}
