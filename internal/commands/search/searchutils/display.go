package searchutils

import (
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

// DisplayIssues formats a list of issues as a table string.
func DisplayIssues(io *iostreams.IOStreams, issues []*gitlab.Issue) string {
	c := io.Color()

	table := tableprinter.NewTablePrinter()
	table.Wrap = false

	if len(issues) > 0 {
		table.AddRow("ID", "Title", "Labels", "Created at")
	}

	for _, issue := range issues {
		labels := ""
		if len(issue.Labels) > 0 {
			labels = fmt.Sprintf("(%s)", strings.Join(issue.Labels, ", "))
		}

		id := c.Green(fmt.Sprintf("#%d", issue.IID))
		if issue.State == "closed" {
			id = c.Red(fmt.Sprintf("#%d", issue.IID))
		}

		createdAt := ""
		if issue.CreatedAt != nil {
			createdAt = utils.TimeToPrettyTimeAgo(*issue.CreatedAt)
		}

		table.AddCell(id)
		table.AddCell(issue.Title)
		table.AddCell(labels)
		table.AddCell(createdAt)
		table.EndRow()
	}

	return table.Render()
}
