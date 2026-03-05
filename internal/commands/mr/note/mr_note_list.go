package note

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
)

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	var filter, state, filePath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list [<id> | <branch>]",
		Short: "List discussions on a merge request",
		Long: heredoc.Doc(`
			Fetch and display all discussions on a merge request.

			Each discussion shows its 8-character ID prefix, resolution status,
			file position (for diff notes), and all notes in the thread.

			Supports JSON output for scripting.
		`),
		Example: heredoc.Doc(`
			# List all discussions on the current branch's MR
			$ glab mr note list

			# List diff comments only
			$ glab mr note list --filter diff

			# List unresolved discussions
			$ glab mr note list --state unresolved

			# List discussions on a specific file
			$ glab mr note list --file src/main.go

			# JSON output for scripting
			$ glab mr note list --json | jq '.[].notes[].body'

			# List discussions on MR 123
			$ glab mr note list 123
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			mr, repo, err := mrutils.MRFromArgs(cmd.Context(), f, args, "any")
			if err != nil {
				return err
			}

			discussions, err := mrutils.ListAllDiscussions(client, repo.FullName(), mr.IID)
			if err != nil {
				return err
			}

			filtered := filterDiscussions(discussions, filter, state, filePath)

			if jsonOutput {
				enc := json.NewEncoder(f.IO().StdOut)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			return outputHuman(f, filtered)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "all", "Note type: all, general, diff, system.")
	cmd.Flags().StringVar(&state, "state", "all", "Resolution state: all, resolved, unresolved.")
	cmd.Flags().StringVar(&filePath, "file", "", "Show only diff notes on this file path.")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON.")

	return cmd
}

func filterDiscussions(discussions []*gitlab.Discussion, filter, state, filePath string) []*gitlab.Discussion {
	var result []*gitlab.Discussion
	for _, d := range discussions {
		if !matchesFilter(d, filter) {
			continue
		}
		if !matchesState(d, state) {
			continue
		}
		if !matchesFile(d, filePath) {
			continue
		}
		result = append(result, d)
	}
	return result
}

func discussionType(d *gitlab.Discussion) string {
	if len(d.Notes) == 0 {
		return "unknown"
	}
	n := d.Notes[0]
	if n.System {
		return "system"
	}
	if n.Position != nil {
		return "diff"
	}
	return "general"
}

func matchesFilter(d *gitlab.Discussion, filter string) bool {
	if filter == "all" {
		return true
	}
	return discussionType(d) == filter
}

func matchesState(d *gitlab.Discussion, state string) bool {
	if state == "all" {
		return true
	}
	if len(d.Notes) == 0 {
		return false
	}
	first := d.Notes[0]
	if !first.Resolvable {
		return false
	}
	allResolved := true
	for _, n := range d.Notes {
		if n.Resolvable && !n.Resolved {
			allResolved = false
			break
		}
	}
	if state == "resolved" {
		return allResolved
	}
	return !allResolved // unresolved
}

func matchesFile(d *gitlab.Discussion, filePath string) bool {
	if filePath == "" {
		return true
	}
	if len(d.Notes) == 0 || d.Notes[0].Position == nil {
		return false
	}
	pos := d.Notes[0].Position
	return pos.NewPath == filePath || pos.OldPath == filePath
}

func outputHuman(f cmdutils.Factory, discussions []*gitlab.Discussion) error {
	out := f.IO().StdOut
	if len(discussions) == 0 {
		fmt.Fprintln(out, "No discussions found.")
		return nil
	}
	for i, d := range discussions {
		if i > 0 {
			fmt.Fprintln(out)
		}
		printDiscussion(f, d)
	}
	return nil
}

func printDiscussion(f cmdutils.Factory, d *gitlab.Discussion) {
	out := f.IO().StdOut
	prefix := d.ID[:8]
	dtype := discussionType(d)

	var parts []string
	parts = append(parts, fmt.Sprintf("#%s", prefix))

	if len(d.Notes) > 0 && d.Notes[0].Resolvable {
		allResolved := true
		for _, n := range d.Notes {
			if n.Resolvable && !n.Resolved {
				allResolved = false
				break
			}
		}
		if allResolved {
			parts = append(parts, "[RESOLVED]")
		} else {
			parts = append(parts, "[UNRESOLVED]")
		}
	}

	if len(d.Notes) > 0 && d.Notes[0].Position != nil {
		pos := d.Notes[0].Position
		fp := pos.NewPath
		if fp == "" {
			fp = pos.OldPath
		}
		if pos.NewLine > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", fp, pos.NewLine))
		} else if pos.OldLine > 0 {
			parts = append(parts, fmt.Sprintf("%s:~%d", fp, pos.OldLine))
		} else {
			parts = append(parts, fp)
		}
	}

	parts = append(parts, fmt.Sprintf("(%s)", dtype))
	fmt.Fprintln(out, strings.Join(parts, " "))

	for _, n := range d.Notes {
		ts := ""
		if n.CreatedAt != nil {
			ts = n.CreatedAt.Format(time.DateTime)
		}
		body := n.Body
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		body = strings.ReplaceAll(body, "\n", " ")
		fmt.Fprintf(out, "  @%-12s (%s): %s\n", n.Author.Username, ts, body)
	}
}
