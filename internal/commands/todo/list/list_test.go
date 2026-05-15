//go:build !integration

package list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
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

// testTodoJSON mirrors a real /todos response, including the group
// field the SDK's gitlab.Todo misses. Raw JSON so the wrapper
// decode runs end-to-end.
const testTodoJSON = `[{
	"id": 42,
	"action_name": "assigned",
	"target_type": "MergeRequest",
	"target": { "title": "Fix the bug" },
	"project": { "path_with_namespace": "mygroup/myproject" },
	"state": "pending",
	"created_at": "2025-01-01T00:00:00Z",
	"body": "review requested",
	"target_url": "https://example.com/mygroup/myproject/-/merge_requests/1"
}]`

// testGroupTodoJSON covers a group-scoped todo: no project, but a
// group with full_path. The SDK drops this; the wrapper keeps it.
const testGroupTodoJSON = `[{
	"id": 99,
	"action_name": "mentioned",
	"target_type": "Epic",
	"target": { "title": "Platform roadmap" },
	"group": {
		"id": 10,
		"name": "gitlab-org",
		"path": "gitlab-org",
		"full_path": "gitlab-org",
		"web_url": "https://example.com/groups/gitlab-org"
	},
	"state": "pending",
	"created_at": "2025-01-01T00:00:00Z",
	"body": "mention",
	"target_url": "https://example.com/groups/gitlab-org/-/epics/5"
}]`

// newTodoTestFactoryOption stands up an httptest server returning
// body on GET /api/v4/todos and wires the factory at it.
func newTodoTestFactoryOption(t *testing.T, body string) cmdtest.FactoryOption {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/todos" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	gitlabClient, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(srv.URL+"/api/v4"))
	require.NoError(t, err)

	apiClient, err := api.NewClient(
		func(*http.Client) (gitlab.AuthSource, error) {
			return gitlab.AccessTokenAuthSource{Token: "test-token"}, nil
		},
		api.WithGitLabClient(gitlabClient),
	)
	require.NoError(t, err)

	return cmdtest.WithApiClient(apiClient)
}

func TestTodoList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cli      string
		body     string
		contains []string
	}{
		{
			name:     "lists pending todos with project context",
			cli:      "",
			body:     testTodoJSON,
			contains: []string{"42", "assigned", "MergeRequest", "Fix the bug", "mygroup/myproject"},
		},
		{
			name:     "filters by --state=done hit the same endpoint",
			cli:      "--state=done",
			body:     testTodoJSON,
			contains: []string{"Fix the bug"},
		},
		{
			name:     "empty array gives a clean blank render",
			cli:      "",
			body:     `[]`,
			contains: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := cmdtest.SetupCmdForTest(t, NewCmd, false, newTodoTestFactoryOption(t, tc.body))
			out, err := exec(tc.cli)
			require.NoError(t, err)
			for _, want := range tc.contains {
				assert.Contains(t, out.OutBuf.String(), want)
			}
		})
	}
}

// TestTodoList_JSON covers the enriched JSON output: target_type,
// action_name, and group.full_path for group-scoped todos.
func TestTodoList_JSON(t *testing.T) {
	t.Parallel()

	t.Run("project-scoped todo", func(t *testing.T) {
		t.Parallel()
		exec := cmdtest.SetupCmdForTest(t, NewCmd, false, newTodoTestFactoryOption(t, testTodoJSON))
		out, err := exec("--output json")
		require.NoError(t, err)

		got := out.OutBuf.String()
		assert.Contains(t, got, `"id":42`)
		assert.Contains(t, got, `"action_name":"assigned"`)
		assert.Contains(t, got, `"target_type":"MergeRequest"`)
		assert.Contains(t, got, `"path_with_namespace":"mygroup/myproject"`)
		assert.NotContains(t, got, `"group":`, "project-only todo should not carry a group field")
	})

	t.Run("group-scoped todo surfaces full_path", func(t *testing.T) {
		t.Parallel()
		exec := cmdtest.SetupCmdForTest(t, NewCmd, false, newTodoTestFactoryOption(t, testGroupTodoJSON))
		out, err := exec("--output json")
		require.NoError(t, err)

		got := out.OutBuf.String()
		assert.Contains(t, got, `"id":99`)
		assert.Contains(t, got, `"action_name":"mentioned"`)
		assert.Contains(t, got, `"target_type":"Epic"`)
		assert.Contains(t, got, `"full_path":"gitlab-org"`)
	})
}
