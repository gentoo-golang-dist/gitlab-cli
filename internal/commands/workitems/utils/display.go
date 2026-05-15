package utils

import (
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

// StateColor returns the tint function for a work item state. OPEN
// is green, CLOSED is red, anything else passes through unchanged --
// future states surface honestly instead of being mistinted as open.
func StateColor(state string, c *iostreams.ColorPalette) func(string) string {
	switch state {
	case "OPEN":
		return c.Green
	case "CLOSED":
		return c.Red
	default:
		return func(s string) string { return s }
	}
}

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
		stateColor := StateColor(wi.State, c)

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
