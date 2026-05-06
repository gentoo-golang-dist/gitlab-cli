//go:build !integration

package view

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	workitemsapi "gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// TestMCPSafeAnnotation pins the Safe marker.
func TestMCPSafeAnnotation(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := cmdtest.TestIOStreams()
	cmd := NewCmd(cmdtest.NewTestFactory(ios))
	assert.Equal(t, "true", cmd.Annotations[mcpannotations.Safe])
}

// respondWithProjectNode returns one node under project. Pass nil
// to exercise the "not found" path.
func respondWithProjectNode(wi *workitemsapi.WorkItem, assertVars func(gitlab.GraphQLQuery)) func(gitlab.GraphQLQuery, any, ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
	return func(query gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
		if assertVars != nil {
			assertVars(query)
		}
		resp := response.(*workitemsapi.WorkItemsResponse)
		var nodes []workitemsapi.WorkItem
		if wi != nil {
			nodes = []workitemsapi.WorkItem{*wi}
		}
		resp.Data.Project = &workitemsapi.ProjectWorkItems{
			WorkItems: workitemsapi.WorkItemsConnection{Nodes: nodes},
		}
		return &gitlab.Response{}, nil
	}
}

func sampleWorkItem() *workitemsapi.WorkItem {
	wi := workitemsapi.WorkItem{
		IID:          "42",
		Title:        "Plan Q2 roadmap",
		State:        "OPEN",
		Description:  "Long-form details\nacross two lines.",
		CreatedAt:    "2026-04-01",
		UpdatedAt:    "2026-04-15",
		WebURL:       "https://gitlab.com/OWNER/REPO/-/work_items/42",
		Confidential: false,
	}
	wi.WorkItemType.Name = "Issue"
	wi.Author.Username = "author1"
	wi.Assignees.Nodes = []workitemsapi.Assignee{{Username: "alice", Name: "Alice"}}
	wi.Labels.Nodes = []workitemsapi.Label{{Title: "backend"}}
	wi.Milestone = &workitemsapi.Milestone{Title: "Sprint 3", DueDate: "2026-05-01"}
	wi.Status = &workitemsapi.WorkItemStatus{Name: "In progress", Category: "in_progress"}
	wi.Parent = &workitemsapi.Parent{IID: "10", Title: "Roadmap epic", WebURL: "https://example.com/10"}
	wi.DueDate = "2026-05-01"
	wi.Namespace = &workitemsapi.Namespace{FullPath: "gitlab-org/cli"}
	wi.Children = &workitemsapi.WorkItemChildren{
		Count: 2,
		Nodes: []workitemsapi.ChildRef{
			{IID: "43", Title: "Task A", State: "OPEN", WebURL: "https://example.com/43"},
			{IID: "44", Title: "Task B", State: "CLOSED", WebURL: "https://example.com/44"},
		},
	}
	wi.Children.Nodes[0].WorkItemType.Name = "Task"
	wi.Children.Nodes[1].WorkItemType.Name = "Task"
	return &wi
}

func runView(t *testing.T, args string, setupMock func(tc *gitlabtesting.TestClient)) (string, error) {
	t.Helper()
	tc := gitlabtesting.NewTestClient(t)
	setupMock(tc)
	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
	)
	out, err := exec(args)
	if out == nil {
		return "", err
	}
	return out.OutBuf.String(), err
}

func TestWorkItemsView_DefaultText(t *testing.T) {
	out, err := runView(t, "42", func(tc *gitlabtesting.TestClient) {
		tc.MockGraphQL.EXPECT().
			Do(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(respondWithProjectNode(sampleWorkItem(), func(q gitlab.GraphQLQuery) {
				assert.Equal(t, "OWNER/REPO", q.Variables["projectPath"])
				assert.Equal(t, "42", q.Variables["iid"])
			}))
	})
	require.NoError(t, err)

	// Identity line, title, and metadata all surface.
	for _, want := range []string{
		"Issue #42",
		"Plan Q2 roadmap",
		"In progress",
		"alice",
		"backend",
		"Sprint 3",
		"#10 Roadmap epic",
		"2026-05-01",
		"gitlab-org/cli",
		"Description",
		"Long-form details",
		"Children",
		"#43",
		"Task A",
	} {
		assert.Contains(t, out, want, "expected %q in output", want)
	}
}

func TestWorkItemsView_JSONOutput(t *testing.T) {
	out, err := runView(t, "42 --output json", func(tc *gitlabtesting.TestClient) {
		tc.MockGraphQL.EXPECT().
			Do(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(respondWithProjectNode(sampleWorkItem(), nil))
	})
	require.NoError(t, err)

	var got workitemsapi.WorkItem
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "42", got.IID)
	assert.Equal(t, "Plan Q2 roadmap", got.Title)
	require.NotNil(t, got.Status)
	assert.Equal(t, "In progress", got.Status.Name)
	require.NotNil(t, got.Children)
	assert.Equal(t, 2, got.Children.Count)
}

func TestWorkItemsView_NotFound(t *testing.T) {
	_, err := runView(t, "9999", func(tc *gitlabtesting.TestClient) {
		tc.MockGraphQL.EXPECT().
			Do(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(respondWithProjectNode(nil, nil))
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work item not found")
}

func TestWorkItemsView_MissingArg(t *testing.T) {
	_, err := runView(t, "", func(_ *gitlabtesting.TestClient) {})
	require.Error(t, err)
}
