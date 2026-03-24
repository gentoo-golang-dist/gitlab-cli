package note

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

// readNoteBody reads the note body from the -m flag, stdin (non-TTY), or editor prompt.
// Returns the trimmed body, or an error if blank.
func readNoteBody(f cmdutils.Factory, cmd *cobra.Command) (string, error) {
	body, _ := cmd.Flags().GetString("message")

	if strings.TrimSpace(body) == "" {
		if !f.IO().IsInTTY {
			data, err := io.ReadAll(f.IO().In)
			if err != nil {
				return "", fmt.Errorf("failed to read from stdin: %w", err)
			}
			body = strings.TrimSpace(string(data))
		} else {
			editor, err := cmdutils.GetEditor(f.Config)
			if err != nil {
				return "", err
			}

			err = f.IO().Editor(cmd.Context(), &body, "Note message:", "Enter the note message for the merge request.", "", editor)
			if err != nil {
				return "", err
			}
		}
	}

	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("aborted... Note has an empty message.")
	}

	return body, nil
}

// deduplicateNote checks whether a note with the same body already exists on the MR.
// If a duplicate is found, it prints the URL and returns (true, nil).
// If no duplicate is found, returns (false, nil).
func deduplicateNote(client *gitlab.Client, repo string, mrIID int64, body, webURL string, out io.Writer) (bool, error) {
	opts := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: api.DefaultListLimit}}
	for {
		notes, resp, err := client.Notes.ListMergeRequestNotes(repo, mrIID, opts)
		if err != nil {
			return false, fmt.Errorf("running merge request note deduplication: %v", err)
		}
		for _, noteInfo := range notes {
			if noteInfo.Body == strings.TrimSpace(body) {
				fmt.Fprintf(out, "%s#note_%d\n", webURL, noteInfo.ID)
				return true, nil
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return false, nil
}
