//go:build !integration

package dsl

import (
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

func TestDsl_HappyPath_JSON(t *testing.T) {
	t.Parallel()
	// GIVEN the Orbit service returns the DSL JSON Schema (format=raw)
	body := `{"$schema":"https://json-schema.org/draft/2020-12/schema","title":"QueryDSL","type":"object"}`
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetDsl(gomock.Any(), gomock.Any()).
		Return(body, &gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote dsl` runs
	out, err := exec("")

	// THEN the bytes are echoed verbatim with a trailing newline. Use a
	// byte-exact assertion (not JSONEq) to lock in pass-through semantics
	// — the command must not reformat or parse the response.
	require.NoError(t, err)
	assert.Equal(t, body+"\n", out.OutBuf.String())
}

func TestDsl_HappyPath_NonJSON(t *testing.T) {
	t.Parallel()
	// GIVEN the Orbit service returns a non-JSON body (e.g. TOON for
	// format=llm). The command must forward bytes verbatim regardless.
	body := "QueryDSL v2.1.0:\nquery_type: traversal | search | aggregation"
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetDsl(gomock.Any(), gomock.Any()).
		Return(body, &gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote dsl` runs
	out, err := exec("")

	// THEN the TOON bytes are echoed verbatim, no JSON parsing attempted
	require.NoError(t, err)
	assert.Equal(t, body+"\n", out.OutBuf.String())
}

func TestDsl_RateLimited(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 429
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetDsl(gomock.Any(), gomock.Any()).
		Return("",
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
