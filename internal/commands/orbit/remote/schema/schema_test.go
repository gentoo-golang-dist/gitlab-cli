//go:build !integration

package schema

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

func TestSchema_NoArgs(t *testing.T) {
	t.Parallel()
	// GIVEN no positional args means no expand option is sent
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.Nil()).
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

	// WHEN `glab orbit schema` runs without arguments
	out, err := exec("")

	// THEN the schema is printed as JSON
	require.NoError(t, err)
	assert.Contains(t, out.OutBuf.String(), `"schema_version":"1.0"`)
	assert.Contains(t, out.OutBuf.String(), `"core"`)
}

func TestSchema_WithExpandPositional(t *testing.T) {
	t.Parallel()
	// GIVEN positional arguments are passed through as the expand list
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.AssignableToTypeOf(&gitlab.GetOrbitSchemaOptions{})).
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

	// WHEN `glab orbit schema User Project MergeRequest` runs
	_, err := exec("User Project MergeRequest")

	// THEN no error and the expected expand list is forwarded
	require.NoError(t, err)
}

func TestSchema_Forbidden(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 403
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetSchema(gomock.Any()).
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
