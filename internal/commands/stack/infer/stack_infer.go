package infer

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/sha3"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/text"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

type options struct {
	stackName string
}

func NewCmdInferStack(f cmdutils.Factory, gr git.GitRunner, getText cmdutils.GetTextUsingEditor) *cobra.Command {
	o := &options{}

	stackInferCmd := &cobra.Command{
		Use:   "infer <revision-range>",
		Short: `Add layers to a stack based on a range of commits. (EXPERIMENTAL.)`,
		Long: `Add layers to a stack based on a range of commits.
This will append layers to an existing stack, or create a new one if needed.
` + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Commit range syntax is similar to "git rev-list":

			## Infer stack from commits between main and current branch
			$ glab stack infer main..HEAD

			## Infer stack from last 5 commits
			$ glab stack infer HEAD~5..HEAD

			## Infer stack from specific commit range
			$ glab stack infer abc123..def456

			## Create a new stack with a specific name
			$ glab stack infer --name feature-stack HEAD~3..HEAD
		`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), f, getText, gr, args, o)
		},
	}

	stackInferCmd.Flags().StringVarP(&o.stackName, "name", "n", "", "Name for the new stack (used when creating a stack)")

	return stackInferCmd
}

func run(ctx context.Context, f cmdutils.Factory, getText cmdutils.GetTextUsingEditor, gr git.GitRunner, args []string, o *options) error {
	// check if in a stack
	title, err := git.GetCurrentStackTitle()
	if err != nil {
		title, err = promptAndCreateStack(ctx, f, gr, o)
		if err != nil {
			return fmt.Errorf("could not create new stack: %v", err)
		}
	}

	stack, err := git.GatherStackRefs(title)
	if err != nil {
		stack = git.Stack{Title: title, Refs: make(map[string]git.StackRef)}
	}

	io := f.IO()
	color := io.Color()

	io.StopSpinner("")
	// pausing the spinner in case it's a terminal based editor

	commits, err := promptForCommits(ctx, f, getText, gr, args)
	if err != nil {
		return fmt.Errorf("error getting commits for stack: %v", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits selected for stack")
	}

	io.StartSpinner("Creating stack layers...")
	defer io.StopSpinner("")

	err = createBranches(f, gr, commits, title, stack)
	if err != nil {
		return fmt.Errorf("error creating stack layers: %v", err)
	}

	io.StopSpinner("")
	fmt.Fprintf(io.StdOut, "%s Added %d layer(s) to stack %q. Run `glab stack sync` to push and create merge requests.\n",
		color.GreenCheck(), len(commits), title)

	return nil
}

func createBranches(f cmdutils.Factory, gr git.GitRunner, commits []string, title string, stack git.Stack) error {
	author, err := git.GitUserName()
	if err != nil {
		return fmt.Errorf("error getting Git author: %v", err)
	}

	var prevSHA string
	if !stack.Empty() {
		prevSHA = stack.Last().SHA
	}

	for _, commitHash := range commits {
		description, err := commitSubject(gr, commitHash)
		if err != nil {
			return fmt.Errorf("error getting commit subject for %s: %v", commitHash, err)
		}

		stackSHA, err := generateStackSha(description, title, string(author), time.Now())
		if err != nil {
			return fmt.Errorf("error generating stack SHA: %v", err)
		}

		branchName, err := createShaBranch(f, stackSHA, title)
		if err != nil {
			return fmt.Errorf("error creating branch name: %v", err)
		}

		_, err = gr.Git("branch", branchName, commitHash)
		if err != nil {
			return fmt.Errorf("error creating branch %s at %s: %v", branchName, commitHash, err)
		}

		if prevSHA != "" {
			prevRef := stack.Refs[prevSHA]
			prevRef.Next = stackSHA
			err = git.UpdateStackRefFile(title, prevRef)
			if err != nil {
				return fmt.Errorf("error updating previous ref: %v", err)
			}
			stack.Refs[prevSHA] = prevRef
		}

		newRef := git.StackRef{
			Prev:        prevSHA,
			SHA:         stackSHA,
			Branch:      branchName,
			Description: description,
		}

		err = git.AddStackRefFile(title, newRef)
		if err != nil {
			return fmt.Errorf("error creating stack ref file: %v", err)
		}

		stack.Refs[stackSHA] = newRef
		prevSHA = stackSHA
	}

	return nil
}

func commitSubject(gr git.GitRunner, hash string) (string, error) {
	output, err := gr.Git("log", "-1", "--format=%s", hash)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func generateStackSha(message string, title string, author string, timestamp time.Time) (string, error) {
	toSha := []byte(message + title + author + timestamp.String())
	hashData := make([]byte, 4)

	shakeHash := sha3.NewShake256()
	shakeHash.Write(toSha)
	_, err := shakeHash.Read(hashData)
	if err != nil {
		return "", fmt.Errorf("error generating hash for stack branch: %v", err)
	}

	return hex.EncodeToString(hashData), nil
}

func createShaBranch(f cmdutils.Factory, sha string, title string) (string, error) {
	cfg := f.Config()

	prefix, err := cfg.Get("", "branch_prefix")
	if err != nil {
		return "", fmt.Errorf("could not get prefix config: %v", err)
	}

	if prefix == "" {
		prefix = os.Getenv("USER")
		if prefix == "" {
			prefix = "glab-stack"
		}
	}

	branchTitle := []string{prefix, title, sha}
	branch := strings.Join(branchTitle, "-")
	return branch, nil
}

// promptAndCreateStack creates a new stack with the provided name or prompts for one
func promptAndCreateStack(ctx context.Context, f cmdutils.Factory, gr git.GitRunner, o *options) (string, error) {
	var titleString string

	if o.stackName != "" {
		titleString = o.stackName
	} else {
		if !f.IO().IsOutputTTY() {
			return "", fmt.Errorf("no stack found and no TTY available. Use --name to specify a stack name")
		}
		err := f.IO().Input(ctx, &titleString, "No stack found. Enter a name for a new stack:", "", func(s string) error {
			if s == "" {
				return fmt.Errorf("title is required")
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("error prompting for title: %v", err)
		}
	}

	io := f.IO()
	color := io.Color()

	title := utils.ReplaceNonAlphaNumericChars(titleString, "-")
	if title != titleString {
		fmt.Fprintf(io.StdErr, "%s warning: invalid characters have been replaced with dashes: %s\n",
			color.WarnIcon(),
			color.Blue(title))
	}

	err := git.SetLocalConfig("glab.currentstack", title)
	if err != nil {
		return "", fmt.Errorf("error setting local Git config: %v", err)
	}

	_, err = git.AddStackRefDir(title)
	if err != nil {
		return "", fmt.Errorf("error adding stack metadata directory: %v", err)
	}

	baseBranch, err := gr.Git("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("error determining current branch: %v", err)
	}

	err = git.AddStackBaseBranch(title, baseBranch)
	if err != nil {
		return "", fmt.Errorf("error adding base branch to metadata: %v", err)
	}

	fmt.Fprintf(io.StdOut, "New stack created with title %q.\n", title)

	return title, nil
}
