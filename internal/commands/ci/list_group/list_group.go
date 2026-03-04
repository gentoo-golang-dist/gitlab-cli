package listgroup

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

func NewCmdListGroup(f cmdutils.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-group <group> [flags]",
		Short: "List pipelines across all projects in a group.",
		Example: heredoc.Doc(`
			# List all pipelines for a group
			$ glab ci list-group my-group

			# List only failed pipelines
			$ glab ci list-group my-group --status failed

			# List pipelines for a deeply nested group
			$ glab ci list-group my-org/team/frontend

			# Include subgroup projects
			$ glab ci list-group my-group --include-subgroups

			# JSON output
			$ glab ci list-group my-group -F json

			# Filter by ref and username
			$ glab ci list-group my-group --ref main --username admin

			# Show duration and triggered-by user
			$ glab ci list-group my-group --detail

			# Show all pipelines without limit
			$ glab ci list-group my-group -P 5 --limit 0
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			groupPath := args[0]

			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			status, _ := cmd.Flags().GetString("status")
			ref, _ := cmd.Flags().GetString("ref")
			username, _ := cmd.Flags().GetString("username")
			source, _ := cmd.Flags().GetString("source")
			perPage, _ := cmd.Flags().GetInt("per-page")
			includeSubgroups, _ := cmd.Flags().GetBool("include-subgroups")
			format, _ := cmd.Flags().GetString("output")

			var updatedAfter, updatedBefore *time.Time
			if m, _ := cmd.Flags().GetString("updated-after"); m != "" {
				t, err := time.Parse("2006-01-02T15:04:05Z", m)
				if err != nil {
					return fmt.Errorf("invalid --updated-after format: %w", err)
				}
				updatedAfter = &t
			}
			if m, _ := cmd.Flags().GetString("updated-before"); m != "" {
				t, err := time.Parse("2006-01-02T15:04:05Z", m)
				if err != nil {
					return fmt.Errorf("invalid --updated-before format: %w", err)
				}
				updatedBefore = &t
			}

			// 1. List all projects in the group
			projects, err := listAllGroupProjects(client, groupPath, includeSubgroups)
			if err != nil {
				return fmt.Errorf("list group projects: %w", err)
			}

			if len(projects) == 0 {
				fmt.Fprintf(f.IO().StdOut, "No projects found in group %s\n", groupPath)
				return nil
			}

			// 2. Fetch latest pipeline(s) per project concurrently
			if format != "json" {
				fmt.Fprintf(f.IO().StdOut, "Fetching pipelines for %d projects...\n", len(projects))
			}
			pipeOpts := &gitlab.ListProjectPipelinesOptions{
				ListOptions: gitlab.ListOptions{PerPage: int64(perPage)},
			}
			if status != "" {
				pipeOpts.Status = gitlab.Ptr(gitlab.BuildStateValue(status))
			}
			if ref != "" {
				pipeOpts.Ref = gitlab.Ptr(ref)
			}
			if username != "" {
				pipeOpts.Username = gitlab.Ptr(username)
			}
			if source != "" {
				pipeOpts.Source = gitlab.Ptr(source)
			}
			if updatedAfter != nil {
				pipeOpts.UpdatedAfter = updatedAfter
			}
			if updatedBefore != nil {
				pipeOpts.UpdatedBefore = updatedBefore
			}

			type projectPipelines struct {
				project   *gitlab.Project
				pipelines []*gitlab.PipelineInfo
			}

			results := make([]projectPipelines, len(projects))
			var wg sync.WaitGroup
			sem := make(chan struct{}, 20) // limit concurrency

			for i, proj := range projects {
				wg.Add(1)
				go func(idx int, p *gitlab.Project) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					pipes, _, _ := client.Pipelines.ListProjectPipelines(p.ID, pipeOpts)
					results[idx] = projectPipelines{project: p, pipelines: pipes}
				}(i, proj)
			}
			wg.Wait()

			// 3. Flatten and sort by updated_at (newest first)
			var rows []pipelineRow
			for _, r := range results {
				for _, p := range r.pipelines {
					rows = append(rows, pipelineRow{
						ProjectPath: r.project.PathWithNamespace,
						ProjectID:   r.project.ID,
						Pipeline:    p,
					})
				}
			}

			sort.Slice(rows, func(i, j int) bool {
				ti := rows[i].Pipeline.UpdatedAt
				tj := rows[j].Pipeline.UpdatedAt
				if ti == nil {
					return false
				}
				if tj == nil {
					return true
				}
				return ti.After(*tj)
			})

			// Apply limit
			limit, _ := cmd.Flags().GetInt("limit")
			totalFound := len(rows)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			// 4. Optionally enrich with full pipeline details (duration, user)
			detail, _ := cmd.Flags().GetBool("detail")
			if detail && len(rows) > 0 {
				if format != "json" {
					fmt.Fprintf(f.IO().StdOut, "Fetching details for %d pipelines...\n", len(rows))
				}
				enrichPipelines(client, rows)
			}

			if format == "json" {
				data, _ := json.Marshal(rows)
				fmt.Fprintln(f.IO().StdOut, string(data))
				return nil
			}

			if len(rows) == 0 {
				fmt.Fprintf(f.IO().StdOut, "No pipelines found across %d projects in group %s\n", len(projects), groupPath)
				return nil
			}

			fmt.Fprintf(f.IO().StdOut, "Showing pipelines across %d projects in group %s", len(projects), groupPath)
			if limit > 0 && totalFound > limit {
				fmt.Fprintf(f.IO().StdOut, " (showing %d of %d, use --limit to adjust)", limit, totalFound)
			}
			fmt.Fprintln(f.IO().StdOut)
			fmt.Fprintln(f.IO().StdOut)
			displayGroupPipelines(f.IO(), rows, detail)
			return nil
		},
	}

	cmd.Flags().StringP("status", "s", "", "Filter by status: running, pending, success, failed, canceled, skipped, created, manual.")
	cmd.Flags().StringP("ref", "r", "", "Filter by ref (branch or tag).")
	cmd.Flags().IntP("per-page", "P", 1, "Number of pipelines per project (default: 1, latest only).")
	cmd.Flags().StringP("output", "F", "text", "Output format: text, json.")
	cmd.Flags().StringP("username", "u", "", "Filter by username of the trigger.")
	cmd.Flags().String("source", "", "Filter by pipeline source.")
	cmd.Flags().StringP("updated-after", "a", "", "Return pipelines updated after date (ISO 8601: 2019-03-15T08:00:00Z).")
	cmd.Flags().StringP("updated-before", "b", "", "Return pipelines updated before date (ISO 8601: 2019-03-15T08:00:00Z).")
	cmd.Flags().Bool("include-subgroups", false, "Include projects from subgroups.")
	cmd.Flags().BoolP("detail", "d", false, "Show duration and triggered-by user (requires extra API calls).")
	cmd.Flags().IntP("limit", "l", 100, "Maximum number of pipelines to show (0 for unlimited).")

	return cmd
}

func listAllGroupProjects(client *gitlab.Client, groupPath string, includeSubgroups bool) ([]*gitlab.Project, error) {
	opts := &gitlab.ListGroupProjectsOptions{
		ListOptions:      gitlab.ListOptions{PerPage: 100},
		IncludeSubGroups: gitlab.Ptr(includeSubgroups),
		OrderBy:          gitlab.Ptr("last_activity_at"),
	}

	var allProjects []*gitlab.Project
	for {
		projects, resp, err := client.Groups.ListGroupProjects(groupPath, opts)
		if err != nil {
			return nil, err
		}
		allProjects = append(allProjects, projects...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = int64(resp.NextPage)
	}
	return allProjects, nil
}

func displayGroupPipelines(ios *iostreams.IOStreams, rows []pipelineRow, detail bool) {
	c := ios.Color()
	table := tableprinter.NewTablePrinter()
	if detail {
		table.AddRow("Status", "Pipeline", "Project", "Ref", "Duration", "User", "Updated")
	} else {
		table.AddRow("Status", "Pipeline", "Project", "Ref", "Updated")
	}

	for _, r := range rows {
		p := r.Pipeline
		var stateStr string
		switch p.Status {
		case "success":
			stateStr = c.Green(p.Status)
		case "failed":
			stateStr = c.Red(p.Status)
		case "running":
			stateStr = c.Blue(p.Status)
		case "pending", "created", "waiting_for_resource", "preparing":
			stateStr = c.Yellow(p.Status)
		case "canceled", "skipped":
			stateStr = c.Gray(p.Status)
		default:
			stateStr = c.Gray(p.Status)
		}

		age := ""
		if p.UpdatedAt != nil {
			age = utils.TimeToPrettyTimeAgo(*p.UpdatedAt)
		}

		pipeID := ios.Hyperlink(fmt.Sprintf("#%d", p.ID), p.WebURL)

		if detail {
			duration := ""
			if r.Detail != nil {
				if r.Detail.Duration > 0 {
					duration = formatDuration(r.Detail.Duration)
				} else if r.Detail.StartedAt != nil {
					// still running — show elapsed time
					duration = formatDuration(int64(time.Since(*r.Detail.StartedAt).Seconds())) + " (running)"
				}
			}
			user := ""
			if r.Detail != nil && r.Detail.User != nil {
				user = r.Detail.User.Username
			}
			table.AddRow(stateStr, pipeID, r.ProjectPath, p.Ref, duration, user, age)
		} else {
			table.AddRow(stateStr, pipeID, r.ProjectPath, p.Ref, age)
		}
	}

	fmt.Fprintln(ios.StdOut, table.Render())
}

// enrichPipelines fetches full pipeline details concurrently.
func enrichPipelines(client *gitlab.Client, rows []pipelineRow) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)

	for i := range rows {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			full, _, err := client.Pipelines.GetPipeline(rows[idx].ProjectID, rows[idx].Pipeline.ID)
			if err == nil {
				rows[idx].Detail = full
			}
		}(i)
	}
	wg.Wait()
}

// formatDuration formats seconds into a human-readable string.
func formatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	return d.Truncate(time.Second).String()
}

type pipelineRow struct {
	ProjectPath string               `json:"project"`
	ProjectID   int64                `json:"project_id,omitempty"`
	Pipeline    *gitlab.PipelineInfo `json:"pipeline"`
	Detail      *gitlab.Pipeline     `json:"detail,omitempty"`
}
