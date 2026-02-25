package utils

import (
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

// DisplayWorkItemList formats and displays work items as a table
func DisplayWorkItemList(streams *iostreams.IOStreams, workItems []api.WorkItem) string {
	if len(workItems) == 0 {
		return ""
	}

	c := streams.Color()
	table := tableprinter.NewTablePrinter()
	table.SetIsTTY(streams.IsOutputTTY())

	table.AddRow("TYPE", "IID", "TITLE", "STATE", "AUTHOR")

	for _, wi := range workItems {
		// evaluate color funcs once per work item
		var stateColor func(string) string
		switch wi.State {
		case "OPEN":
			stateColor = c.Green
		case "CLOSED":
			stateColor = c.Red
		default:
			stateColor = func(s string) string { return s } // no color
		}

		// TYPE column
		table.AddCell(wi.WorkItemType.Name)

		// IID column with hyperlink and color based on state
		iidText := wi.IID
		coloredIID := stateColor(iidText)
		table.AddCell(streams.Hyperlink(coloredIID, wi.WebURL))

		// TITLE column
		table.AddCell(wi.Title)

		// STATE column (colored)
		table.AddCell(stateColor(wi.State))

		// AUTHOR colukmn
		table.AddCell(wi.Author.Username)

		table.EndRow()
	}

	return table.Render()
}
