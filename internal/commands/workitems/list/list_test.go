//go:build !integration

package list

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

// TestMCPSafeAnnotation pins the Safe marker; losing it drops the
// tool from the MCP surface silently.
func TestMCPSafeAnnotation(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := cmdtest.TestIOStreams()
	cmd := NewCmd(cmdtest.NewTestFactory(ios))
	assert.Equal(t, "true", cmd.Annotations[mcpannotations.Safe])
}

// buildWorkItem keeps test literals short. Pass what matters;
// option funcs cover the rest.
func buildWorkItem(iid, title, state, itemType, author string, opts ...func(*workitemsapi.WorkItem)) workitemsapi.WorkItem {
	wi := workitemsapi.WorkItem{
		IID:    iid,
		Title:  title,
		State:  state,
		WebURL: "https://gitlab.com/OWNER/REPO/-/work_items/" + iid,
	}
	wi.WorkItemType.Name = itemType
	wi.Author.Username = author
	for _, opt := range opts {
		opt(&wi)
	}
	return wi
}

func withAssignees(assignees ...workitemsapi.Assignee) func(*workitemsapi.WorkItem) {
	return func(wi *workitemsapi.WorkItem) { wi.Assignees.Nodes = assignees }
}

func withLabels(labels ...workitemsapi.Label) func(*workitemsapi.WorkItem) {
	return func(wi *workitemsapi.WorkItem) { wi.Labels.Nodes = labels }
}

func withMilestone(m workitemsapi.Milestone) func(*workitemsapi.WorkItem) {
	return func(wi *workitemsapi.WorkItem) { wi.Milestone = &m }
}

func withTimestamps(created, updated string) func(*workitemsapi.WorkItem) {
	return func(wi *workitemsapi.WorkItem) {
		wi.CreatedAt = created
		wi.UpdatedAt = updated
	}
}

// respondWithProject drops the DoAndReturn boilerplate from tests.
func respondWithProject(nodes []workitemsapi.WorkItem, pageInfo workitemsapi.PageInfo, assertVars func(gitlab.GraphQLQuery)) func(gitlab.GraphQLQuery, any, ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
	return func(query gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
		if assertVars != nil {
			assertVars(query)
		}
		resp := response.(*workitemsapi.WorkItemsResponse)
		resp.Data.Project = &workitemsapi.ProjectWorkItems{
			WorkItems: workitemsapi.WorkItemsConnection{
				Nodes:    nodes,
				PageInfo: pageInfo,
			},
		}
		return &gitlab.Response{}, nil
	}
}

func respondWithGroup(nodes []workitemsapi.WorkItem, pageInfo workitemsapi.PageInfo) func(gitlab.GraphQLQuery, any, ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
	return func(_ gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
		resp := response.(*workitemsapi.WorkItemsResponse)
		resp.Data.Group = &workitemsapi.GroupWorkItems{
			WorkItems: workitemsapi.WorkItemsConnection{
				Nodes:    nodes,
				PageInfo: pageInfo,
			},
		}
		return &gitlab.Response{}, nil
	}
}

func respondWithCurrentUser(nodes []workitemsapi.WorkItem, pageInfo workitemsapi.PageInfo, assertVars func(gitlab.GraphQLQuery)) func(gitlab.GraphQLQuery, any, ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
	return func(query gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
		if assertVars != nil {
			assertVars(query)
		}
		resp := response.(*workitemsapi.WorkItemsResponse)
		resp.Data.CurrentUser = &workitemsapi.CurrentUserWorkItems{
			WorkItems: workitemsapi.WorkItemsConnection{
				Nodes:    nodes,
				PageInfo: pageInfo,
			},
		}
		return &gitlab.Response{}, nil
	}
}

// testUsername is the shared authenticated-user handle. Using one
// value keeps assertions about "the caller" vs someone else clear.
const testUsername = "jhebden"

// respondWithMe answers the username-probe query. --mine tests use
// it as the first mocked call.
func respondWithMe() func(gitlab.GraphQLQuery, any, ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
	return func(_ gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
		resp := response.(*workitemsapi.CurrentUserIdentity)
		resp.Data.CurrentUser = &workitemsapi.CurrentUserInfo{Username: testUsername}
		return &gitlab.Response{}, nil
	}
}

func TestWorkItemsList(t *testing.T) {
	noPageInfo := workitemsapi.PageInfo{HasNextPage: false}

	tests := []struct {
		name       string
		args       string
		setupMock  func(tc *gitlabtesting.TestClient)
		wantErr    bool
		wantOutput string
	}{
		{
			name: "lists work items in project",
			args: "",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Implement new feature", "OPEN", "Issue", "testuser"),
							buildWorkItem("2", "Fix critical bug", "CLOSED", "Issue", "anotheruser"),
						},
						noPageInfo,
						nil,
					))
			},
			wantOutput: "Implement new feature",
		},
		{
			name: "lists work items in group",
			args: "--group test-group",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithGroup(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Epic for Q1", "OPEN", "Epic", "groupowner"),
						},
						noPageInfo,
					))
			},
			wantOutput: "Epic for Q1",
		},
		{
			name: "empty work items list",
			args: "",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(nil, noPageInfo, nil))
			},
			wantOutput: "No work items found in OWNER/REPO",
		},
		{
			name: "filters by work item type",
			args: "--type epic",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Q1 Planning Epic", "OPEN", "Epic", "epicowner"),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.Contains(t, q.Variables, "types")
							types, ok := q.Variables["types"].([]string)
							require.True(t, ok)
							assert.Equal(t, []string{"EPIC"}, types)
						},
					))
			},
			wantOutput: "Q1 Planning Epic",
		},
		{
			name: "json output format",
			args: "--output json",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Test Item", "OPEN", "Issue", "testuser"),
						},
						noPageInfo,
						nil,
					))
			},
			wantOutput: `"iid": "1"`,
		},
		{
			name: "handles pagination with cursor",
			args: "--after cursor123",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("2", "Second page item", "OPEN", "Issue", "user2"),
						},
						workitemsapi.PageInfo{EndCursor: "cursor456", HasNextPage: true},
						func(q gitlab.GraphQLQuery) {
							assert.Equal(t, "cursor123", q.Variables["after"])
						},
					))
			},
			wantOutput: `Next page: glab work-items list --after "cursor456"`,
		},
		{
			name: "filters by state - closed",
			args: "--state closed",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Closed issue", "CLOSED", "Issue", "testuser"),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.Equal(t, "closed", q.Variables["state"])
						},
					))
			},
			wantOutput: "Closed issue",
		},
		{
			name: "filters by state - all (no state filter sent)",
			args: "--state all",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Open issue", "OPEN", "Issue", "user1"),
							buildWorkItem("2", "Closed issue", "CLOSED", "Issue", "user2"),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.NotContains(t, q.Variables, "state")
						},
					))
			},
			wantOutput: "Open issue",
		},
		{
			name: "default state is opened",
			args: "",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Open issue", "OPEN", "Issue", "user1"),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.Equal(t, "opened", q.Variables["state"])
						},
					))
			},
			wantOutput: "Open issue",
		},
		{
			name:    "handles API error",
			args:    "",
			wantErr: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
		},
		{
			name:    "handles project not found",
			args:    "",
			wantErr: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ gitlab.GraphQLQuery, response any, _ ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
						resp := response.(*workitemsapi.WorkItemsResponse)
						resp.Data.Project = nil
						return &gitlab.Response{}, nil
					})
			},
		},
		{
			name: "filters by assignee username",
			args: "--assignee johndoe",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("5", "Assigned to johndoe", "OPEN", "Issue", "author1",
								withAssignees(workitemsapi.Assignee{Username: "johndoe", Name: "John Doe"})),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.Contains(t, q.Variables, "assigneeUsernames")
							assignees, ok := q.Variables["assigneeUsernames"].([]string)
							require.True(t, ok)
							assert.Equal(t, []string{"johndoe"}, assignees)
						},
					))
			},
			wantOutput: "Assigned to johndoe",
		},
		{
			name: "filters by multiple assignee usernames",
			args: "--assignee alice,bob",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						nil,
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assignees, ok := q.Variables["assigneeUsernames"].([]string)
							require.True(t, ok)
							assert.Equal(t, []string{"alice", "bob"}, assignees)
						},
					))
			},
			wantOutput: "No work items found",
		},
		{
			name: "no assignee filter when flag not set",
			args: "",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("1", "Unfiltered item", "OPEN", "Issue", "user1"),
						},
						noPageInfo,
						func(q gitlab.GraphQLQuery) {
							assert.NotContains(t, q.Variables, "assigneeUsernames")
						},
					))
			},
			wantOutput: "Unfiltered item",
		},
		{
			name: "assignee flag preserved in next page command",
			args: "--assignee johndoe --after cursor123",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(respondWithProject(
						[]workitemsapi.WorkItem{
							buildWorkItem("3", "Page two item", "OPEN", "Issue", "user1"),
						},
						workitemsapi.PageInfo{EndCursor: "cursorNext", HasNextPage: true},
						nil,
					))
			},
			wantOutput: "--assignee johndoe",
		},
		{
			name: "--mine in a repo prepends caller to assignees and keeps project scope",
			args: "--mine",
			setupMock: func(tc *gitlabtesting.TestClient) {
				gomock.InOrder(
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithMe()),
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithProject(
							[]workitemsapi.WorkItem{
								buildWorkItem("7", "My item", "OPEN", "Issue", "somebody",
									withAssignees(workitemsapi.Assignee{Username: "jhebden", Name: "James"})),
							},
							noPageInfo,
							func(q gitlab.GraphQLQuery) {
								assignees, ok := q.Variables["assigneeUsernames"].([]string)
								require.True(t, ok)
								assert.Equal(t, []string{"jhebden"}, assignees)
								// In-repo invocation picks the project query,
								// so projectPath is present.
								assert.Equal(t, "OWNER/REPO", q.Variables["projectPath"])
							},
						)),
				)
			},
			wantOutput: "My item",
		},
		{
			name: "--mine --assignee alice merges caller with explicit assignee",
			args: "--mine --assignee alice",
			setupMock: func(tc *gitlabtesting.TestClient) {
				gomock.InOrder(
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithMe()),
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithProject(
							nil,
							noPageInfo,
							func(q gitlab.GraphQLQuery) {
								assignees, ok := q.Variables["assigneeUsernames"].([]string)
								require.True(t, ok)
								// me always leads; alice follows.
								assert.Equal(t, []string{"jhebden", "alice"}, assignees)
							},
						)),
				)
			},
			wantOutput: "No work items found",
		},
		{
			name: "--mine preserved in next-page command",
			args: "--mine --after cursor123",
			setupMock: func(tc *gitlabtesting.TestClient) {
				gomock.InOrder(
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithMe()),
					tc.MockGraphQL.EXPECT().
						Do(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(respondWithProject(
							[]workitemsapi.WorkItem{
								buildWorkItem("9", "Paged item", "OPEN", "Issue", "user1"),
							},
							workitemsapi.PageInfo{EndCursor: "cursorNext", HasNextPage: true},
							nil,
						)),
				)
			},
			wantOutput: "--mine",
		},
		{
			name:    "--mine surfaces a username resolution failure",
			args:    "--mine",
			wantErr: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockGraphQL.EXPECT().
					Do(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := gitlabtesting.NewTestClient(t)
			tt.setupMock(tc)

			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmd,
				false,
				cmdtest.WithGitLabClient(tc.Client),
				cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
			)

			out, err := exec(tt.args)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, out.OutBuf.String(), tt.wantOutput)
			}
		})
	}
}

// TestWorkItemsList_MineFallsBackToCurrentUserScope: --mine outside
// a repo flips to the cross-namespace currentUser query.
func TestWorkItemsList_MineFallsBackToCurrentUserScope(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)

	gomock.InOrder(
		tc.MockGraphQL.EXPECT().
			Do(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(respondWithMe()),
		tc.MockGraphQL.EXPECT().
			Do(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(respondWithCurrentUser(
				[]workitemsapi.WorkItem{
					buildWorkItem("1", "Cross-namespace item", "OPEN", "Issue", "somebody",
						withAssignees(workitemsapi.Assignee{Username: "jhebden"})),
				},
				workitemsapi.PageInfo{HasNextPage: false},
				func(q gitlab.GraphQLQuery) {
					// currentUser query takes no path variable.
					assert.NotContains(t, q.Variables, "projectPath")
					assert.NotContains(t, q.Variables, "groupPath")
					assignees, ok := q.Variables["assigneeUsernames"].([]string)
					require.True(t, ok)
					assert.Equal(t, []string{"jhebden"}, assignees)
				},
			)),
	)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBaseRepoError(assert.AnError),
	)

	out, err := exec("--mine")
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), "Cross-namespace item")
}

// TestWorkItemsList_NoMineStillRequiresScope: without --mine and no
// scope, the command still fails fast.
func TestWorkItemsList_NoMineStillRequiresScope(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)
	// No GraphQL calls expected — the error happens before any network I/O.

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBaseRepoError(assert.AnError),
	)

	_, err := exec("")
	require.Error(t, err)
}

// TestWorkItemsList_JSONIncludesEnrichedFields proves assignees,
// labels, milestone, and timestamps land in --output json.
func TestWorkItemsList_JSONIncludesEnrichedFields(t *testing.T) {
	tc := gitlabtesting.NewTestClient(t)

	item := buildWorkItem("42", "Planning work", "OPEN", "Issue", "author1",
		withAssignees(workitemsapi.Assignee{Username: "alice", Name: "Alice"}),
		withLabels(workitemsapi.Label{Title: "backend"}, workitemsapi.Label{Title: "p1"}),
		withMilestone(workitemsapi.Milestone{Title: "Sprint 3", DueDate: "2026-05-01"}),
		withTimestamps("2026-04-01T10:00:00Z", "2026-04-15T09:30:00Z"),
	)
	tc.MockGraphQL.EXPECT().
		Do(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(respondWithProject([]workitemsapi.WorkItem{item}, workitemsapi.PageInfo{HasNextPage: false}, nil))

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithGitLabClient(tc.Client),
		cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
	)

	out, err := exec("--output json")
	require.NoError(t, err)

	var decoded struct {
		Data []workitemsapi.WorkItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &decoded))
	require.Len(t, decoded.Data, 1)

	got := decoded.Data[0]
	assert.Equal(t, []workitemsapi.Assignee{{Username: "alice", Name: "Alice"}}, got.Assignees.Nodes)
	assert.Equal(t, []workitemsapi.Label{{Title: "backend"}, {Title: "p1"}}, got.Labels.Nodes)
	require.NotNil(t, got.Milestone)
	assert.Equal(t, "Sprint 3", got.Milestone.Title)
	assert.Equal(t, "2026-05-01", got.Milestone.DueDate)
	assert.Equal(t, "2026-04-01T10:00:00Z", got.CreatedAt)
	assert.Equal(t, "2026-04-15T09:30:00Z", got.UpdatedAt)
}

func TestWorkItemsList_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{
			name:    "invalid output format",
			args:    "--output xml",
			wantErr: "must be one of",
		},
		{
			name:    "empty type in comma-separated list",
			args:    "--type epic,",
			wantErr: "empty work item type",
		},
		{
			name:    "whitespace only type",
			args:    "--type '  '",
			wantErr: "empty work item type",
		},
		{
			name:    "invalid per-page value - too high",
			args:    "--per-page 101",
			wantErr: "--per-page must be between 1 and 100",
		},
		{
			name:    "invalid per-page value - too low",
			args:    "--per-page 0",
			wantErr: "--per-page must be between 1 and 100",
		},
		{
			name:    "invalid page value",
			args:    "--page 0",
			wantErr: "unknown flag: --page",
		},
		{
			name:    "invalid state value",
			args:    "--state invalid",
			wantErr: "--state must be one of: opened, closed, all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmd,
				false,
				cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
			)

			_, err := exec(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
