//go:build !integration

package tools

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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
		GetTools(gomock.Any()).
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

	// WHEN `glab orbit remote tools` runs
	out, err := exec("")

	// THEN the tool list is printed as a JSON array with expected fields
	require.NoError(t, err)

	var result []*gitlab.OrbitTool
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &result))
	require.Len(t, result, 2)
	assert.Equal(t, "query_graph", result[0].Name)
	assert.Equal(t, "get_graph_schema", result[1].Name)
}

func TestTools_RateLimited(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 429
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetTools(gomock.Any()).
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
