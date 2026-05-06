package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// buildWorkItem keeps test literals short.
func buildWorkItem(iid, title, state, itemType, author string, assignees []api.Assignee) api.WorkItem {
	wi := api.WorkItem{
		IID:    iid,
		Title:  title,
		State:  state,
		WebURL: "https://example.com/" + iid,
	}
	wi.WorkItemType.Name = itemType
	wi.Author.Username = author
	wi.Assignees.Nodes = assignees
	return wi
}

// withStatus sets an explicit status widget for STATUS-column tests.
func withStatus(name, category string) func(*api.WorkItem) {
	return func(wi *api.WorkItem) {
		wi.Status = &api.WorkItemStatus{Name: name, Category: category}
	}
}

func withNamespace(fullPath string) func(*api.WorkItem) {
	return func(wi *api.WorkItem) { wi.Namespace = &api.Namespace{FullPath: fullPath} }
}

func withDueDate(date string) func(*api.WorkItem) {
	return func(wi *api.WorkItem) { wi.DueDate = date }
}

func withParent(iid, title string) func(*api.WorkItem) {
	return func(wi *api.WorkItem) {
		wi.Parent = &api.Parent{IID: iid, Title: title, WebURL: "https://example.com/parent/" + iid}
	}
}

func with(wi api.WorkItem, opts ...func(*api.WorkItem)) api.WorkItem {
	for _, opt := range opts {
		opt(&wi)
	}
	return wi
}

func withBlocked() func(*api.WorkItem) {
	return func(wi *api.WorkItem) { wi.Blocked = true }
}

func TestDisplayWorkItemList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		workItems     []api.WorkItem
		isTTY         bool
		expectedParts []string
		notExpected   []string
	}{
		{
			name:          "empty list",
			workItems:     []api.WorkItem{},
			isTTY:         false,
			expectedParts: []string{},
		},
		{
			name:  "single open work item shows 'new' status by default",
			isTTY: true,
			workItems: []api.WorkItem{
				buildWorkItem("123", "Test Epic", "OPEN", "Epic", "testuser", nil),
			},
			expectedParts: []string{"TYPE", "IID", "STATUS", "Epic", "123", "Test Epic", "new", "testuser"},
		},
		{
			name:  "open and closed items pick default status labels",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("1", "Open Epic", "OPEN", "Epic", "user1", nil),
				buildWorkItem("2", "Closed Issue", "CLOSED", "Issue", "user2", nil),
			},
			expectedParts: []string{"Open Epic", "Closed Issue", "new", "completed", "Epic", "Issue"},
			notExpected:   []string{"OPEN", "CLOSED"},
		},
		{
			name:  "multiple types displayed",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("10", "Epic Item", "OPEN", "Epic", "author1", nil),
				buildWorkItem("20", "Task Item", "OPEN", "Task", "author2", nil),
			},
			expectedParts: []string{"Epic", "Task", "Epic Item", "Task Item"},
		},
		{
			name:  "shows assignees column header and single assignee",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("1", "Item with assignee", "OPEN", "Issue", "author1",
					[]api.Assignee{{Username: "assignee1", Name: "Assignee One"}}),
			},
			expectedParts: []string{"ASSIGNEES", "assignee1"},
		},
		{
			name:  "shows dash when no assignees",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("2", "Unassigned item", "OPEN", "Issue", "author1", nil),
			},
			expectedParts: []string{"ASSIGNEES", "-"},
		},
		{
			name:  "shows multiple assignees comma-separated",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("3", "Shared item", "OPEN", "Issue", "author1",
					[]api.Assignee{
						{Username: "alice", Name: "Alice"},
						{Username: "bob", Name: "Bob"},
					}),
			},
			expectedParts: []string{"alice, bob"},
		},
		{
			name:  "explicit 'In progress' status overrides the default label",
			isTTY: false,
			workItems: func() []api.WorkItem {
				wi := buildWorkItem("11", "Active task", "OPEN", "Task", "dev1", nil)
				withStatus("In progress", "in_progress")(&wi)
				return []api.WorkItem{wi}
			}(),
			expectedParts: []string{"In progress", "Active task"},
			notExpected:   []string{"new"},
		},
		{
			name:  "explicit 'Won't do' on a closed item renders verbatim",
			isTTY: false,
			workItems: func() []api.WorkItem {
				wi := buildWorkItem("12", "Rejected task", "CLOSED", "Task", "dev1", nil)
				withStatus("Won't do", "canceled")(&wi)
				return []api.WorkItem{wi}
			}(),
			expectedParts: []string{"Won't do", "Rejected task"},
			notExpected:   []string{"completed"},
		},
		{
			name:  "status widget with empty name still falls back to default",
			isTTY: false,
			workItems: func() []api.WorkItem {
				wi := buildWorkItem("13", "Edge case", "OPEN", "Issue", "dev1", nil)
				withStatus("", "")(&wi)
				return []api.WorkItem{wi}
			}(),
			expectedParts: []string{"new", "Edge case"},
		},
		{
			name:  "single-namespace result hides NAMESPACE column",
			isTTY: false,
			workItems: []api.WorkItem{
				with(buildWorkItem("1", "Same-ns A", "OPEN", "Issue", "u1", nil),
					withNamespace("gitlab-org/cli")),
				with(buildWorkItem("2", "Same-ns B", "OPEN", "Issue", "u2", nil),
					withNamespace("gitlab-org/cli")),
			},
			expectedParts: []string{"Same-ns A", "Same-ns B"},
			notExpected:   []string{"NAMESPACE"},
		},
		{
			name:  "cross-namespace result surfaces NAMESPACE column",
			isTTY: false,
			workItems: []api.WorkItem{
				with(buildWorkItem("1", "From CLI", "OPEN", "Issue", "u1", nil),
					withNamespace("gitlab-org/cli")),
				with(buildWorkItem("2", "From Gitaly", "OPEN", "Issue", "u2", nil),
					withNamespace("gitlab-org/gitaly")),
			},
			expectedParts: []string{"NAMESPACE", "gitlab-org/cli", "gitlab-org/gitaly"},
		},
		{
			name:  "DUE DATE column appears when any item has one",
			isTTY: false,
			workItems: []api.WorkItem{
				with(buildWorkItem("1", "With due", "OPEN", "Issue", "u1", nil),
					withDueDate("2026-05-01")),
				buildWorkItem("2", "Without due", "OPEN", "Issue", "u2", nil),
			},
			expectedParts: []string{"DUE DATE", "2026-05-01", "Without due"},
		},
		{
			name:  "PARENT column truncates overly long titles",
			isTTY: false,
			workItems: []api.WorkItem{
				with(buildWorkItem("1", "Child task", "OPEN", "Task", "u1", nil),
					withParent("42", "A really long parent epic title that exceeds the cap")),
			},
			expectedParts: []string{"PARENT", "A really long parent epi…"},
		},
		{
			name:  "blocked item carries [blocked] tag in its title cell",
			isTTY: false,
			workItems: []api.WorkItem{
				with(buildWorkItem("50", "Waiting on upstream", "OPEN", "Issue", "u1", nil),
					withBlocked()),
			},
			expectedParts: []string{"Waiting on upstream", "[blocked]"},
		},
		{
			name:  "non-blocked item has no [blocked] tag",
			isTTY: false,
			workItems: []api.WorkItem{
				buildWorkItem("51", "Moving along", "OPEN", "Issue", "u1", nil),
			},
			expectedParts: []string{"Moving along"},
			notExpected:   []string{"[blocked]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(tt.isTTY))

			output := DisplayWorkItemList(ios, tt.workItems)

			if len(tt.expectedParts) == 0 {
				assert.Empty(t, output)
			} else {
				for _, part := range tt.expectedParts {
					assert.Contains(t, output, part)
				}
			}

			for _, part := range tt.notExpected {
				assert.NotContains(t, output, part)
			}
		})
	}
}

// TestStatusLabel covers the fallback logic without a table render.
func TestStatusLabel(t *testing.T) {
	t.Parallel()

	mkWI := func(state string, status *api.WorkItemStatus) api.WorkItem {
		wi := api.WorkItem{State: state}
		wi.Status = status
		return wi
	}

	cases := []struct {
		name string
		in   api.WorkItem
		want string
	}{
		{"open no status falls back to 'new'", mkWI("OPEN", nil), defaultOpenStatus},
		{"closed no status falls back to 'completed'", mkWI("CLOSED", nil), defaultClosedStatus},
		{"explicit status wins over default", mkWI("OPEN", &api.WorkItemStatus{Name: "In progress"}), "In progress"},
		{"explicit status wins on closed items too", mkWI("CLOSED", &api.WorkItemStatus{Name: "Won't do"}), "Won't do"},
		{"empty name treated as unset", mkWI("OPEN", &api.WorkItemStatus{Name: "", Category: "triage"}), defaultOpenStatus},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, statusLabel(tc.in))
		})
	}
}

// TestPickColumns covers the optional-column decision logic.
func TestPickColumns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		items []api.WorkItem
		want  columns
	}{
		{
			name:  "empty input hides every optional column",
			items: nil,
			want:  columns{},
		},
		{
			name: "single namespace hides NAMESPACE",
			items: []api.WorkItem{
				with(api.WorkItem{}, withNamespace("gitlab-org/cli")),
				with(api.WorkItem{}, withNamespace("gitlab-org/cli")),
			},
			want: columns{namespace: false},
		},
		{
			name: "two namespaces show NAMESPACE",
			items: []api.WorkItem{
				with(api.WorkItem{}, withNamespace("gitlab-org/cli")),
				with(api.WorkItem{}, withNamespace("gitlab-org/rails")),
			},
			want: columns{namespace: true},
		},
		{
			name: "any due date switches DUE DATE on",
			items: []api.WorkItem{
				{},
				with(api.WorkItem{}, withDueDate("2026-05-01")),
			},
			want: columns{dueDate: true},
		},
		{
			name: "any parent switches PARENT on",
			items: []api.WorkItem{
				{},
				with(api.WorkItem{}, withParent("10", "Parent title")),
			},
			want: columns{parent: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, pickColumns(tc.items))
		})
	}
}

// TestTruncate covers rune-aware truncation (NAMESPACE, PARENT).
func TestTruncate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-ten", 11, "exactly-ten"},
		{"exceeds limit by a lot", 10, "exceeds l…"},
		{"tiny", 1, "…"},
		// Multi-byte: byte counting would split a rune.
		{"日本語のタイトルが長いです", 6, "日本語のタ…"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, truncate(tc.in, tc.max))
		})
	}
}

// TestOrDash pins the placeholder: an empty string breaks column
// alignment, so the helper always returns a dash.
func TestOrDash(t *testing.T) {
	t.Parallel()
	assert.Equal(t, unassignedCell, orDash(""))
	assert.Equal(t, "2026-05-01", orDash("2026-05-01"))
}

// TestFormatAssignees covers the helper directly.
func TestFormatAssignees(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []api.Assignee
		want string
	}{
		{"nil", nil, unassignedCell},
		{"empty", []api.Assignee{}, unassignedCell},
		{"one", []api.Assignee{{Username: "alice"}}, "alice"},
		{"many preserves order", []api.Assignee{
			{Username: "alice"}, {Username: "bob"}, {Username: "carol"},
		}, "alice, bob, carol"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatAssignees(tc.in))
		})
	}
}
