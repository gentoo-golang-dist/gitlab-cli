//go:build !integration

package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkItemUnmarshalFlattensWidgets decodes a GitLab-shaped
// payload and asserts Assignees / Labels / Milestone end up flat
// on WorkItem. Widget order is mixed; the parser shouldn't care.
func TestWorkItemUnmarshalFlattensWidgets(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "42",
		"title": "Plan Q2 roadmap",
		"state": "OPEN",
		"workItemType": { "name": "Issue" },
		"author": { "username": "author1" },
		"widgets": [
			{ "type": "LABELS", "labels": { "nodes": [ { "title": "backend" }, { "title": "p1" } ] } },
			{ "type": "ASSIGNEES", "assignees": { "nodes": [ { "username": "alice", "name": "Alice" } ] } },
			{ "type": "MILESTONE", "milestone": { "title": "Sprint 3", "dueDate": "2026-05-01" } },
			{ "type": "DESCRIPTION" }
		],
		"createdAt": "2026-04-01T10:00:00Z",
		"updatedAt": "2026-04-15T09:30:00Z",
		"webUrl": "https://gitlab.com/OWNER/REPO/-/work_items/42"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))

	assert.Equal(t, "42", wi.IID)
	assert.Equal(t, "Plan Q2 roadmap", wi.Title)
	assert.Equal(t, "OPEN", wi.State)

	assert.Equal(t, []Assignee{{Username: "alice", Name: "Alice"}}, wi.Assignees.Nodes)
	assert.Equal(t, []Label{{Title: "backend"}, {Title: "p1"}}, wi.Labels.Nodes)
	require.NotNil(t, wi.Milestone)
	assert.Equal(t, "Sprint 3", wi.Milestone.Title)
	assert.Equal(t, "2026-05-01", wi.Milestone.DueDate)
}

// TestWorkItemUnmarshalEmptyWidgets covers items that don't carry
// our widgets (e.g. epics with no ASSIGNEES). Flat fields stay at
// their zero values.
func TestWorkItemUnmarshalEmptyWidgets(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "7",
		"title": "Epic without assignees",
		"state": "OPEN",
		"workItemType": { "name": "Epic" },
		"author": { "username": "owner" },
		"widgets": [],
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-01T00:00:00Z",
		"webUrl": "https://example.com/7"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))

	assert.Empty(t, wi.Assignees.Nodes)
	assert.Empty(t, wi.Labels.Nodes)
	assert.Nil(t, wi.Milestone)
}

// TestWorkItemUnmarshalPlanningWidgets covers START_AND_DUE_DATE
// and HIERARCHY flattening, plus the top-level Namespace round-trip
// through the alias decoder.
func TestWorkItemUnmarshalPlanningWidgets(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "10",
		"title": "Planning-heavy item",
		"state": "OPEN",
		"workItemType": { "name": "Issue" },
		"author": { "username": "author" },
		"namespace": { "fullPath": "gitlab-org/cli" },
		"widgets": [
			{ "type": "START_AND_DUE_DATE", "dueDate": "2026-05-01", "startDate": "2026-04-15" },
			{ "type": "HIERARCHY", "parent": { "iid": "42", "title": "Parent epic", "webUrl": "https://example.com/parent/42" } }
		],
		"createdAt": "2026-04-01T00:00:00Z",
		"updatedAt": "2026-04-15T00:00:00Z",
		"webUrl": "https://example.com/10"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))

	require.NotNil(t, wi.Namespace)
	assert.Equal(t, "gitlab-org/cli", wi.Namespace.FullPath)
	assert.Equal(t, "2026-05-01", wi.DueDate)
	assert.Equal(t, "2026-04-15", wi.StartDate)

	require.NotNil(t, wi.Parent)
	assert.Equal(t, "42", wi.Parent.IID)
	assert.Equal(t, "Parent epic", wi.Parent.Title)
	assert.Equal(t, "https://example.com/parent/42", wi.Parent.WebURL)
}

// TestWorkItemUnmarshalStatusWidget covers the STATUS widget: name
// and category land on WorkItem.Status.
func TestWorkItemUnmarshalStatusWidget(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "1",
		"title": "Status-bearing item",
		"state": "OPEN",
		"workItemType": { "name": "Task" },
		"author": { "username": "author" },
		"widgets": [
			{ "type": "STATUS", "status": { "name": "In progress", "category": "in_progress" } }
		],
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-01T00:00:00Z",
		"webUrl": "https://example.com/1"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))
	require.NotNil(t, wi.Status)
	assert.Equal(t, "In progress", wi.Status.Name)
	assert.Equal(t, "in_progress", wi.Status.Category)
}

// TestWorkItemUnmarshalEEWidgets covers LINKED_ITEMS and
// HEALTH_STATUS flattening. On CE these widgets are absent and the
// flat fields stay zero, covered by the other tests.
func TestWorkItemUnmarshalEEWidgets(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "1",
		"title": "Blocked item",
		"state": "OPEN",
		"workItemType": { "name": "Issue" },
		"author": { "username": "author" },
		"widgets": [
			{ "type": "LINKED_ITEMS", "blocked": true, "blockedByCount": 2, "blockingCount": 0 },
			{ "type": "HEALTH_STATUS", "healthStatus": "atRisk" }
		],
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-01T00:00:00Z",
		"webUrl": "https://example.com/1"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))
	assert.True(t, wi.Blocked)
	assert.Equal(t, 2, wi.BlockedByCount)
	assert.Equal(t, 0, wi.BlockingCount)
	assert.Equal(t, "atRisk", wi.HealthStatus)
}

// TestWorkItemUnmarshalMissingWidgetsField covers a response with
// no widgets key. Go should decode cleanly with zero values.
func TestWorkItemUnmarshalMissingWidgetsField(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"iid": "1",
		"title": "Sparse response",
		"state": "OPEN",
		"workItemType": { "name": "Issue" },
		"author": { "username": "author" },
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-01T00:00:00Z",
		"webUrl": "https://example.com/1"
	}`)

	var wi WorkItem
	require.NoError(t, json.Unmarshal(payload, &wi))
	assert.Empty(t, wi.Assignees.Nodes)
	assert.Empty(t, wi.Labels.Nodes)
	assert.Nil(t, wi.Milestone)
}
