package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// scope type constants
const (
	ScopeTypeGroup   = "group"
	ScopeTypeProject = "project"
)

// ScopeInfo contains detected scope information for work items queries
type ScopeInfo struct {
	Type string
	Path string
}

// Assignee is a single assignee on a work item.
type Assignee struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

// Label is a single label applied to a work item.
type Label struct {
	Title string `json:"title"`
}

// Milestone is the milestone a work item belongs to.
type Milestone struct {
	Title   string `json:"title"`
	DueDate string `json:"dueDate"`
}

// Namespace is the group or project the work item lives in. Only
// fullPath is queried; nothing else drives the planning output.
type Namespace struct {
	FullPath string `json:"fullPath"`
}

// Parent is the parent work item from the HIERARCHY widget.
type Parent struct {
	IID    string `json:"iid"`
	Title  string `json:"title"`
	WebURL string `json:"webUrl"`
}

// ChildRef is a single child from the HIERARCHY widget. Lighter than
// WorkItem: grandchildren don't get fetched.
type ChildRef struct {
	IID          string `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	WorkItemType struct {
		Name string `json:"name"`
	} `json:"workItemType"`
	WebURL string `json:"webUrl"`
}

// WorkItemChildren wraps the children array. Count is the server
// total; Nodes is the first page.
type WorkItemChildren struct {
	Count int        `json:"count"`
	Nodes []ChildRef `json:"nodes"`
}

// WorkItem is a GraphQL work item. GitLab returns assignees, labels,
// milestone, and similar fields inside a "widgets" array;
// UnmarshalJSON flattens them onto the top level so callers see a
// clean shape.
type WorkItem struct {
	IID          string `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	WorkItemType struct {
		Name string `json:"name"`
	} `json:"workItemType"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	Assignees struct {
		Nodes []Assignee `json:"nodes"`
	} `json:"assignees"`
	Labels struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
	Milestone *Milestone      `json:"milestone,omitempty"`
	Status    *WorkItemStatus `json:"status,omitempty"`
	Namespace *Namespace      `json:"namespace,omitempty"`
	DueDate   string          `json:"dueDate,omitempty"`
	StartDate string          `json:"startDate,omitempty"`
	Parent    *Parent         `json:"parent,omitempty"`
	// EE-only fields; see the capability probe in ee.go.
	Blocked        bool   `json:"blocked,omitempty"`
	BlockedByCount int    `json:"blockedByCount,omitempty"`
	BlockingCount  int    `json:"blockingCount,omitempty"`
	HealthStatus   string `json:"healthStatus,omitempty"`
	// Detail-view fields; list queries don't select these.
	Description  string            `json:"description,omitempty"`
	Confidential bool              `json:"confidential,omitempty"`
	ClosedAt     string            `json:"closedAt,omitempty"`
	Children     *WorkItemChildren `json:"children,omitempty"`
	CreatedAt    string            `json:"createdAt"`
	UpdatedAt    string            `json:"updatedAt"`
	WebURL       string            `json:"webUrl"`
}

// workItemWire carries the widgets array for UnmarshalJSON to walk.
type workItemWire struct {
	Widgets []workItemWidget `json:"widgets"`
}

// workItemWidget holds every widget field the query asks for. Only
// the one matching Type is populated on a given entry.
type workItemWidget struct {
	Type      string `json:"type"`
	Assignees *struct {
		Nodes []Assignee `json:"nodes"`
	} `json:"assignees,omitempty"`
	Labels *struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels,omitempty"`
	Milestone      *Milestone        `json:"milestone,omitempty"`
	Status         *WorkItemStatus   `json:"status,omitempty"`
	DueDate        string            `json:"dueDate,omitempty"`
	StartDate      string            `json:"startDate,omitempty"`
	Parent         *Parent           `json:"parent,omitempty"`
	Children       *WorkItemChildren `json:"children,omitempty"`
	Blocked        bool              `json:"blocked,omitempty"`
	BlockedByCount int               `json:"blockedByCount,omitempty"`
	BlockingCount  int               `json:"blockingCount,omitempty"`
	HealthStatus   string            `json:"healthStatus,omitempty"`
}

// UnmarshalJSON decodes top-level fields normally, then walks the
// widgets array and copies each value onto its flat equivalent.
func (wi *WorkItem) UnmarshalJSON(data []byte) error {
	// alias avoids recursing into this method.
	type alias WorkItem
	aux := struct {
		*alias
		workItemWire
	}{alias: (*alias)(wi)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	for _, w := range aux.Widgets {
		switch w.Type {
		case WidgetTypeAssignees:
			if w.Assignees != nil {
				wi.Assignees.Nodes = w.Assignees.Nodes
			}
		case WidgetTypeLabels:
			if w.Labels != nil {
				wi.Labels.Nodes = w.Labels.Nodes
			}
		case WidgetTypeMilestone:
			wi.Milestone = w.Milestone
		case WidgetTypeStatus:
			wi.Status = w.Status
		case WidgetTypeStartAndDueDate:
			wi.DueDate = w.DueDate
			wi.StartDate = w.StartDate
		case WidgetTypeHierarchy:
			wi.Parent = w.Parent
			if w.Children != nil {
				wi.Children = w.Children
			}
		case WidgetTypeLinkedItems:
			wi.Blocked = w.Blocked
			wi.BlockedByCount = w.BlockedByCount
			wi.BlockingCount = w.BlockingCount
		case WidgetTypeHealthStatus:
			wi.HealthStatus = w.HealthStatus
		}
	}
	return nil
}

// Widget type discriminators from GitLab's WorkItemWidget union.
const (
	WidgetTypeAssignees       = "ASSIGNEES"
	WidgetTypeLabels          = "LABELS"
	WidgetTypeMilestone       = "MILESTONE"
	WidgetTypeStatus          = "STATUS"
	WidgetTypeStartAndDueDate = "START_AND_DUE_DATE"
	WidgetTypeHierarchy       = "HIERARCHY"
	WidgetTypeLinkedItems     = "LINKED_ITEMS"
	WidgetTypeHealthStatus    = "HEALTH_STATUS"
)

// WorkItemStatus is the value of the status widget. System-defined
// names are "To do", "In progress", "Done", "Won't do", "Duplicate";
// namespaces can add custom statuses. Category is one of triage,
// to_do, in_progress, done, canceled.
type WorkItemStatus struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

// eeWidgetSpreads covers Premium/Ultimate-only widgets. On CE these
// types don't exist and the fragment fails validation; the
// capability probe in ee.go strips this block on retry.
const eeWidgetSpreads = `
	... on WorkItemWidgetStatus {
		status { name category }
	}
	... on WorkItemWidgetLinkedItems {
		blocked
		blockedByCount
		blockingCount
	}
	... on WorkItemWidgetHealthStatus {
		healthStatus
	}
`

// GraphQL query templates for list. Unchanged from the original
// flat-field form -- list pagination doesn't need the widget data
// the view command pulls.
const (
	groupWorkItemsQuery = `
	query ListGroupWorkItems($groupPath: ID!, $types: [IssueType!], $state: IssuableState, $first: Int, $after: String) {
	group(fullPath: $groupPath) {
		workItems(types: $types, state: $state, first: $first, after: $after) {
			nodes {
				iid
				title
				state
				workItemType {
					name
				}
				author {
					username
				}
				webUrl
			}
			pageInfo {
				endCursor
				hasNextPage
			}
		}
	}
}
`

	projectWorkItemsQuery = `
	query ListProjectWorkItems($projectPath: ID!, $types: [IssueType!], $state: IssuableState, $first: Int, $after: String) {
	project(fullPath: $projectPath) {
		workItems(types: $types, state: $state, first: $first, after: $after) {
			nodes {
				iid
				title
				state
				workItemType {
					name
				}
				author {
					username
				}
				webUrl
			}
			pageInfo {
				endCursor
				hasNextPage
			}
		}
	}
}
`
)

// FetchWorkItems retrieves all work items using cursor-based pagination
func FetchWorkItems(ctx context.Context, client *gitlab.Client, scope *ScopeInfo, types []string, state string, after string, perPage int64) ([]WorkItem, *PageInfo, error) {
	var queryStr string
	var pathKey string

	switch scope.Type {
	case ScopeTypeGroup:
		queryStr = groupWorkItemsQuery
		pathKey = "groupPath"
	case ScopeTypeProject:
		queryStr = projectWorkItemsQuery
		pathKey = "projectPath"
	default:
		return nil, nil, fmt.Errorf("invalid scope type: %s", scope.Type)
	}

	// uppercase types for GraphQL API (API exepcts EPIC not epic)
	var uppercaseTypes []string
	if len(types) > 0 {
		uppercaseTypes = make([]string, len(types))
		for i, t := range types {
			uppercaseTypes[i] = strings.ToUpper(strings.TrimSpace(t))
		}
	}

	// Build query vars
	variables := map[string]any{
		pathKey: scope.Path,
		"first": perPage, // user-specified page size (max 100)
	}

	if after != "" {
		variables["after"] = after
	}

	// add types filter if specified
	if len(uppercaseTypes) > 0 {
		variables["types"] = uppercaseTypes
	}

	// add state filter
	if state != "all" {
		variables["state"] = state
	}

	query := gitlab.GraphQLQuery{
		Query:     queryStr,
		Variables: variables,
	}

	var response WorkItemsResponse

	// execute query
	_, err := client.GraphQL.Do(query, &response, gitlab.WithContext(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	// Extract work items based on scope
	var nodes []WorkItem
	var pageInfo PageInfo

	if scope.Type == ScopeTypeGroup {
		if response.Data.Group == nil {
			return nil, nil, fmt.Errorf("group not found: %s", scope.Path)
		}
		nodes = response.Data.Group.WorkItems.Nodes
		pageInfo = response.Data.Group.WorkItems.PageInfo
	} else {
		if response.Data.Project == nil {
			return nil, nil, fmt.Errorf("project not found: %s", scope.Path)
		}
		nodes = response.Data.Project.WorkItems.Nodes
		pageInfo = response.Data.Project.WorkItems.PageInfo
	}

	return nodes, &pageInfo, nil
}

// workItemDetailFields is the view-side selection: adds description,
// confidential, closedAt, and hierarchy children. Separate from the
// list selection so pagination doesn't pay the children-count cost.
func workItemDetailFields(includeEE bool) string {
	widgets := ceDetailWidgetSpreads
	if includeEE {
		widgets += eeWidgetSpreads
	}
	return `
iid
title
state
description
confidential
workItemType { name }
author { username }
namespace { fullPath }
widgets {
	type` + widgets + `
}
createdAt
updatedAt
closedAt
webUrl
`
}

// ceDetailWidgetSpreads is the view superset of ceWidgetSpreads;
// HIERARCHY also pulls children here.
const ceDetailWidgetSpreads = `
	... on WorkItemWidgetAssignees {
		assignees { nodes { username name } }
	}
	... on WorkItemWidgetLabels {
		labels { nodes { title } }
	}
	... on WorkItemWidgetMilestone {
		milestone { title dueDate }
	}
	... on WorkItemWidgetStartAndDueDate {
		dueDate
		startDate
	}
	... on WorkItemWidgetHierarchy {
		parent { iid title webUrl }
		children {
			count
			nodes {
				iid
				title
				state
				workItemType { name }
				webUrl
			}
		}
	}
`

func groupWorkItemViewQuery(includeEE bool) string {
	return `
	query GetGroupWorkItem($groupPath: ID!, $iid: String!) {
		group(fullPath: $groupPath) {
			workItems(iid: $iid, first: 1) {
				nodes {` + workItemDetailFields(includeEE) + `}
			}
		}
	}
	`
}

func projectWorkItemViewQuery(includeEE bool) string {
	return `
	query GetProjectWorkItem($projectPath: ID!, $iid: String!) {
		project(fullPath: $projectPath) {
			workItems(iid: $iid, first: 1) {
				nodes {` + workItemDetailFields(includeEE) + `}
			}
		}
	}
	`
}

// FetchWorkItem loads one work item by IID. "Scope not accessible"
// and "iid not found" come back as distinct errors so callers can
// tell them apart.
func FetchWorkItem(ctx context.Context, client *gitlab.Client, scope *ScopeInfo, iid string) (*WorkItem, error) {
	var queryBuilder func(bool) string
	var pathKey string
	switch scope.Type {
	case ScopeTypeGroup:
		queryBuilder, pathKey = groupWorkItemViewQuery, "groupPath"
	case ScopeTypeProject:
		queryBuilder, pathKey = projectWorkItemViewQuery, "projectPath"
	default:
		return nil, fmt.Errorf("view does not support scope type: %s", scope.Type)
	}

	variables := map[string]any{
		pathKey: scope.Path,
		"iid":   iid,
	}

	var response WorkItemsResponse
	if err := doWithEEFallback(ctx, client, &response, func(includeEE bool) gitlab.GraphQLQuery {
		return gitlab.GraphQLQuery{
			Query:     queryBuilder(includeEE),
			Variables: variables,
		}
	}); err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	var conn *WorkItemsConnection
	var label string
	switch scope.Type {
	case ScopeTypeGroup:
		label = "group"
		if response.Data.Group != nil {
			conn = &response.Data.Group.WorkItems
		}
	case ScopeTypeProject:
		label = "project"
		if response.Data.Project != nil {
			conn = &response.Data.Project.WorkItems
		}
	}
	if conn == nil {
		return nil, fmt.Errorf("%s not found: %s", label, scope.Path)
	}
	if len(conn.Nodes) == 0 {
		return nil, fmt.Errorf("work item not found: %s!%s", scope.Path, iid)
	}
	wi := conn.Nodes[0]
	return &wi, nil
}

// WorkItemsResponse represents the GraphQL response structure for work items queries
type WorkItemsResponse struct {
	Data struct {
		Group   *GroupWorkItems   `json:"group,omitempty"`
		Project *ProjectWorkItems `json:"project,omitempty"`
	} `json:"data"`
}

// helper structs for GraphQL response parsing
type GroupWorkItems struct {
	WorkItems WorkItemsConnection `json:"workItems"`
}

type ProjectWorkItems struct {
	WorkItems WorkItemsConnection `json:"workItems"`
}

type WorkItemsConnection struct {
	Nodes    []WorkItem `json:"nodes"`
	PageInfo PageInfo   `json:"pageInfo"`
}

type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}
