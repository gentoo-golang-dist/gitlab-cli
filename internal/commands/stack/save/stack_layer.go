package save

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/sha3"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

var layerDescription string

func NewCmdLayerStack(f cmdutils.Factory, gr git.GitRunner, getText cmdutils.GetTextUsingEditor) *cobra.Command {
	stackLayerCmd := &cobra.Command{
		Use:   "layer",
		Short: `Create a new diff on top of the current stack. (EXPERIMENTAL)`,
		Long: `Explicitly create a new diff (layer) on top of the current stack.

Use this command when you want to add a new change to your stack,
creating a new branch and merge request for it. This is different
from 'amend', which updates the current diff in place.
` + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab stack layer added_file
			$ glab stack layer . -m "added a function"
			$ glab stack layer -m "added a function"`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("message") && cmd.Flags().Changed("description") {
				return &cmdutils.FlagError{Err: errors.New("specify either of --message or --description.")}
			}

			// check if there are even any changes before we start
			err := checkForChanges()
			if err != nil {
				return fmt.Errorf("could not create layer: %v", err)
			}

			// a description is required, so ask if one is not provided
			if layerDescription == "" {
				layerDescription, err = promptForCommit(cmd.Context(), f, getText, "")
				if err != nil {
					return fmt.Errorf("error getting commit message: %v", err)
				}
			}

			s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)

			// git add files
			err = addFiles(args[0:])
			if err != nil {
				return fmt.Errorf("error adding files: %v", err)
			}

			// get stack title
			title, err := git.GetCurrentStackTitle()
			if err != nil {
				return fmt.Errorf("error running Git command: %v", err)
			}

			author, err := git.GitUserName()
			if err != nil {
				return fmt.Errorf("error getting Git author: %v", err)
			}

			// generate a SHA based on: commit message, stack title, Git author name
			sha, err := generateLayerSha(layerDescription, title, string(author), time.Now())
			if err != nil {
				return fmt.Errorf("error generating hash for stack branch name: %v", err)
			}

			// create branch name from SHA
			branch, err := createShaBranch(f, sha, title)
			if err != nil {
				return fmt.Errorf("error creating branch name: %v", err)
			}

			// create the branch prefix-stack_title-SHA
			err = git.CheckoutNewBranch(branch)
			if err != nil {
				return fmt.Errorf("error running branch checkout: %v", err)
			}

			// commit files to branch
			_, err = commitFiles(layerDescription)
			if err != nil {
				return fmt.Errorf("error committing files: %v", err)
			}

			stack, err := git.GatherStackRefs(title)
			if err != nil {
				return fmt.Errorf("error getting refs from file system: %v", err)
			}

			var stackRef git.StackRef
			if !stack.Empty() {
				lastRef := stack.Last()

				// update the ref before it (the current last ref)
				err = git.UpdateStackRefFile(title, git.StackRef{
					Prev:        lastRef.Prev,
					MR:          lastRef.MR,
					Description: lastRef.Description,
					SHA:         lastRef.SHA,
					Branch:      lastRef.Branch,
					Next:        sha,
				})
				if err != nil {
					return fmt.Errorf("error updating old ref: %v", err)
				}

				stackRef = git.StackRef{Prev: lastRef.SHA, SHA: sha, Branch: branch, Description: layerDescription}
			} else {
				stackRef = git.StackRef{SHA: sha, Branch: branch, Description: layerDescription}
			}

			err = git.AddStackRefFile(title, stackRef)
			if err != nil {
				return fmt.Errorf("error creating stack file: %v", err)
			}

			if f.IO().IsOutputTTY() {
				color := f.IO().Color()

				fmt.Fprintf(
					f.IO().StdOut,
					"%s %s: Created new layer with message: \"%s\".\n",
					color.ProgressIcon(),
					color.Blue(title),
					layerDescription,
				)
			}

			s.Stop()

			return nil
		},
	}
	stackLayerCmd.Flags().StringVarP(&layerDescription, "description", "d", "", "Description of the change.")
	stackLayerCmd.Flags().StringVarP(&layerDescription, "message", "m", "", "Alias for the description flag.")

	return stackLayerCmd
}

func generateLayerSha(message string, title string, author string, timestamp time.Time) (string, error) {
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
