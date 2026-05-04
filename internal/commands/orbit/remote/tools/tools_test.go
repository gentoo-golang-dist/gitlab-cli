//go:build !integration

package tools

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestTools_HappyPath(t *testing.T) {
	t.Parallel()
	// GIVEN the Orbit service returns two MCP tool definitions
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetTools().
		Return(&gitlab.OrbitTools{
			Tools: []*gitlab.OrbitTool{
				{
					Name:        "query_graph",
					Description: "Execute graph queries",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
				{Name: "get_graph_schema", Description: "List Knowledge Graph schema"},
			},
		}, &gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit tools` runs
	out, err := exec("")

	// THEN the tool list is printed as a JSON array
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), `"query_graph"`)
	assert.Contains(t, out.OutBuf.String(), `"get_graph_schema"`)
}

func TestTools_RateLimited(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 429
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetTools().
		Return(nil,
			&gitlab.Response{Response: &http.Response{StatusCode: http.StatusTooManyRequests}},
			&gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusTooManyRequests},
				Message:  "Too Many Requests",
			})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN the command runs
	_, err := exec("")

	// THEN the error maps to ExitRateLimited (exit code 5)
	require.Error(t, err)
	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, orbiterr.ExitRateLimited, exitErr.Code)
}
