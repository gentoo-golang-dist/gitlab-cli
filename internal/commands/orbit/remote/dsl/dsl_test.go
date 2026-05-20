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

func TestDsl_HappyPath(t *testing.T) {
	t.Parallel()
	// GIVEN the Orbit service returns the DSL JSON Schema
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

	// THEN the DSL JSON Schema is written to stdout verbatim
	require.NoError(t, err)
	assert.JSONEq(t, body, out.OutBuf.String())
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
