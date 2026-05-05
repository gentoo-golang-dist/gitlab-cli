//go:build !integration

package schema

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

func TestSchema_NoArgs(t *testing.T) {
	t.Parallel()
	// GIVEN no positional args means no expand option is sent
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.Nil(), gomock.Any()).
		Return(&gitlab.OrbitSchema{
			SchemaVersion: "1.0",
			Domains: []*gitlab.OrbitSchemaDomain{
				{Name: "core", NodeNames: []string{"User", "Project"}},
			},
		}, &gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote schema` runs without arguments
	out, err := exec("")

	// THEN the schema is printed as JSON with expected fields
	require.NoError(t, err)

	var result gitlab.OrbitSchema
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &result))
	assert.Equal(t, "1.0", result.SchemaVersion)
	require.Len(t, result.Domains, 1)
	assert.Equal(t, "core", result.Domains[0].Name)
	assert.Equal(t, []string{"User", "Project"}, result.Domains[0].NodeNames)
}

func TestSchema_WithExpandPositional(t *testing.T) {
	t.Parallel()
	// GIVEN positional arguments are passed through as the expand list
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.AssignableToTypeOf(&gitlab.GetOrbitSchemaOptions{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.GetOrbitSchemaOptions, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitSchema, *gitlab.Response, error) {
			require.NotNil(t, opts)
			require.NotNil(t, opts.Expand)
			assert.Equal(t, []string{"User", "Project", "MergeRequest"}, *opts.Expand)
			return &gitlab.OrbitSchema{SchemaVersion: "1.0"},
				&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote schema User Project MergeRequest` runs
	out, err := exec("User Project MergeRequest")

	// THEN no error, the expected expand list is forwarded, and output is valid JSON
	require.NoError(t, err)

	var result gitlab.OrbitSchema
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &result))
	assert.Equal(t, "1.0", result.SchemaVersion)
}

func TestSchema_Forbidden(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 403
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.Any(), gomock.Any()).
		Return(nil,
			&gitlab.Response{Response: &http.Response{StatusCode: http.StatusForbidden}},
			&gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Message:  "No Knowledge Graph enabled namespaces available",
			})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN the command runs
	_, err := exec("")

	// THEN the error is mapped to ExitForbidden (exit code 4)
	require.Error(t, err)
	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, orbiterr.ExitForbidden, exitErr.Code)
}
