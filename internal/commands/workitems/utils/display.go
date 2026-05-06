package utils

import (
	"strings"

	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

// unassignedCell is the placeholder for empty table cells. JSON
// output still sees the original empty value.
const unassignedCell = "-"

// Default STATUS labels when the status widget isn't set.
const (
	defaultOpenStatus   = "new"
	defaultClosedStatus = "completed"
)

// Column widths for the optional planning columns.
const (
	namespaceMaxLen = 30
	parentMaxLen    = 25
)

// DisplayWorkItemList renders work items as a table. Baseline
// columns always show; planning columns (NAMESPACE, DUE DATE,
// PARENT) only appear when at least one row has a value.
func DisplayWorkItemList(streams *iostreams.IOStreams, workItems []api.WorkItem) string {
	if len(workItems) == 0 {
		return ""
	}

	cols := pickColumns(workItems)
	c := streams.Color()
	table := tableprinter.NewTablePrinter()
	table.SetIsTTY(streams.IsOutputTTY())

	header := []string{"TYPE", "IID", "TITLE", "STATUS", "AUTHOR", "ASSIGNEES"}
	if cols.namespace {
		header = append(header, "NAMESPACE")
	}
	if cols.dueDate {
		header = append(header, "DUE DATE")
	}
	if cols.parent {
		header = append(header, "PARENT")
	}
	table.AddRow(toAny(header)...)

	for _, wi := range workItems {
		stateColor := colorForState(c, wi.State)

		table.AddCell(wi.WorkItemType.Name)
		table.AddCell(streams.Hyperlink(stateColor(wi.IID), wi.WebURL))
		table.AddCell(titleCell(c, wi))
		table.AddCell(stateColor(statusLabel(wi)))
		table.AddCell(wi.Author.Username)
		table.AddCell(formatAssignees(wi.Assignees.Nodes))

		if cols.namespace {
			table.AddCell(formatNamespace(wi.Namespace))
		}
		if cols.dueDate {
			table.AddCell(orDash(wi.DueDate))
		}
		if cols.parent {
			table.AddCell(formatParent(wi.Parent))
		}
		table.EndRow()
	}

	return table.Render()
}

// columns records which optional columns the renderer emits.
// Computed once so the header matches every row.
type columns struct {
	namespace bool
	dueDate   bool
	parent    bool
}

// pickColumns toggles each optional column. NAMESPACE only lights up
// when results span more than one namespace.
func pickColumns(items []api.WorkItem) columns {
	var (
		seenNamespaces = map[string]struct{}{}
		hasDue, hasPar bool
	)
	for _, wi := range items {
		if wi.Namespace != nil && wi.Namespace.FullPath != "" {
			seenNamespaces[wi.Namespace.FullPath] = struct{}{}
		}
		if wi.DueDate != "" {
			hasDue = true
		}
		if wi.Parent != nil {
			hasPar = true
		}
	}
	return columns{
		namespace: len(seenNamespaces) > 1,
		dueDate:   hasDue,
		parent:    hasPar,
	}
}

// colorForState returns the tint used for the IID link and STATUS
// cell: green for open, red for closed, no-op otherwise.
func colorForState(c *iostreams.ColorPalette, state string) func(string) string {
	switch state {
	case "OPEN":
		return c.Green
	case "CLOSED":
		return c.Red
	default:
		return func(s string) string { return s }
	}
}

// titleCell appends a red "[blocked]" tag when the EE LinkedItems
// widget flagged the item. CE instances always render the plain title.
func titleCell(c *iostreams.ColorPalette, wi api.WorkItem) string {
	if wi.Blocked {
		return wi.Title + " " + c.Red("[blocked]")
	}
	return wi.Title
}

// statusLabel returns the STATUS cell text. An explicit status widget
// wins; otherwise we fall back to a label derived from state so the
// column is never blank.
func statusLabel(wi api.WorkItem) string {
	if wi.Status != nil && wi.Status.Name != "" {
		return wi.Status.Name
	}
	if wi.State == "CLOSED" {
		return defaultClosedStatus
	}
	return defaultOpenStatus
}

func formatAssignees(assignees []api.Assignee) string {
	if len(assignees) == 0 {
		return unassignedCell
	}
	names := make([]string, len(assignees))
	for i, a := range assignees {
		names[i] = a.Username
	}
	return strings.Join(names, ", ")
}

func formatNamespace(ns *api.Namespace) string {
	if ns == nil || ns.FullPath == "" {
		return unassignedCell
	}
	return truncate(ns.FullPath, namespaceMaxLen)
}

func formatParent(p *api.Parent) string {
	if p == nil || p.Title == "" {
		return unassignedCell
	}
	return truncate(p.Title, parentMaxLen)
}

// truncate shortens s to max runes with an ellipsis. Rune-aware so
// multi-byte characters don't split mid-glyph.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func orDash(s string) string {
	if s == "" {
		return unassignedCell
	}
	return s
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
