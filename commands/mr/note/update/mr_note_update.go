package note

import (
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/cli/commands/mr/mrutils"

	"gitlab.com/gitlab-org/cli/api"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

func UpdateCmdNote(f *cmdutils.Factory) *cobra.Command {
	mrCreateNoteCmd := &cobra.Command{
		Use:   "update [<id> | <branch>] [note-id]",
		Short: "Update a comment or note on a merge request.",
		Long:  ``,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, err := f.HttpClient()
			if err != nil {
				return err
			}

			mr, repo, err := mrutils.MRFromArgs(f, args, "any")
			if err != nil {
				return err
			}

			body, _ := cmd.Flags().GetString("message")

			if body == "" {
				editor, err := cmdutils.GetEditor(f.Config)
				if err != nil {
					return err
				}

				body = utils.Editor(utils.EditorOptions{
					Label:         "Note message:",
					Help:          "Enter the note message for the merge request. ",
					FileName:      "*_MR_NOTE_EDITMSG.md",
					EditorCommand: editor,
				})
			}
			if body == "" {
				return fmt.Errorf("aborted... Note has an empty message.")
			}

			noteId, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}

			if noteId < 0 {
				return fmt.Errorf("aborted... Note ID must not be negative.")
			}

			noteInfo, err := api.UpdateMRNotes(apiClient, repo.FullName(), mr.IID, noteId, &gitlab.UpdateMergeRequestNoteOptions{
				Body: &body,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(f.IO.StdOut, "%s#note_%d\n", mr.WebURL, noteInfo.ID)
			return nil
		},
	}

	mrCreateNoteCmd.Flags().StringP("message", "m", "", "Comment or note message.")
	return mrCreateNoteCmd
}
