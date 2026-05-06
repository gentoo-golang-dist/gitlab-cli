//go:build !integration

package query

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
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

const stdinBody = `{"query":{"query_type":"traversal","node":{"id":"p","entity":"Project"},"limit":1}}`

func TestQuery_Stdin_DefaultsToLLM(t *testing.T) {
	t.Parallel()
	// GIVEN a body on stdin and no --format flag
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.AssignableToTypeOf(&gitlab.OrbitQueryRequest{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.OrbitQueryRequest, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitQueryResult, *gitlab.Response, error) {
			require.NotNil(t, opts)
			require.NotNil(t, opts.ResponseFormat)
			assert.Equal(t, gitlab.OrbitResponseFormatLLM, *opts.ResponseFormat)

			// AND the user-supplied query is forwarded verbatim
			var got map[string]any
			require.NoError(t, json.Unmarshal(opts.Query, &got))
			assert.Equal(t, "traversal", got["query_type"])

			return &gitlab.OrbitQueryResult{
					Result:   json.RawMessage(`"@goon{}"`),
					RowCount: 0,
				},
				&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote query -` runs (stdin)
	out, err := exec("-")

	// THEN no error and the result is printed as JSON
	require.NoError(t, err)

	var result gitlab.OrbitQueryResult
	require.NoError(t, json.Unmarshal(out.OutBuf.Bytes(), &result))
	assert.Contains(t, string(result.Result), "@goon")
}

func TestQuery_FlagOverridesBodyResponseFormat(t *testing.T) {
	t.Parallel()
	// GIVEN the body sets response_format=raw but --format=llm is passed
	body := `{"query":{"query_type":"neighbors","node_ids":[1]},"response_format":"raw"}`

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.AssignableToTypeOf(&gitlab.OrbitQueryRequest{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.OrbitQueryRequest, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitQueryResult, *gitlab.Response, error) {
			require.NotNil(t, opts.ResponseFormat)
			assert.Equal(t, gitlab.OrbitResponseFormatLLM, *opts.ResponseFormat, "--format must override the body's response_format")
			return &gitlab.OrbitQueryResult{},
				&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(body),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN
	_, err := exec("--format llm -")

	// THEN
	require.NoError(t, err)
}

func TestQuery_BodyFormatHonoredWhenNoFlag(t *testing.T) {
	t.Parallel()
	// GIVEN the body sets response_format=raw and --format is NOT passed
	body := `{"query":{"query_type":"neighbors","node_ids":[1]},"response_format":"raw"}`

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.AssignableToTypeOf(&gitlab.OrbitQueryRequest{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.OrbitQueryRequest, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitQueryResult, *gitlab.Response, error) {
			require.NotNil(t, opts.ResponseFormat)
			// The body's response_format must win when --format is absent.
			// Previously the cobra default of "llm" silently overrode it.
			assert.Equal(t, gitlab.OrbitResponseFormatRaw, *opts.ResponseFormat,
				"body's response_format must win when --format is not passed")
			return &gitlab.OrbitQueryResult{},
				&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(body),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN the command runs without --format
	_, err := exec("-")

	// THEN the body's response_format is forwarded to the API
	require.NoError(t, err)
}

func TestQuery_FromFile(t *testing.T) {
	t.Parallel()
	// GIVEN the body lives in a file
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "q.json")
	require.NoError(t, os.WriteFile(bodyPath, []byte(stdinBody), 0o600))

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.Any(), gomock.Any()).
		Return(&gitlab.OrbitQueryResult{},
			&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN `glab orbit remote query <file>` runs
	_, err := exec(bodyPath)

	// THEN no error
	require.NoError(t, err)
}

func TestQuery_InvalidFormatFlag(t *testing.T) {
	t.Parallel()
	// GIVEN a body on stdin and an invalid --format value
	// NewEnumValue rejects unknown values at flag parsing time, before RunE runs.
	testClient := gitlabtesting.NewTestClient(t)
	// no API call expected

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN
	_, err := exec("--format yaml -")

	// THEN cobra rejects the flag before RunE executes
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestQuery_EmptyBody(t *testing.T) {
	t.Parallel()
	// GIVEN empty stdin
	testClient := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(""),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN
	_, err := exec("-")

	// THEN the user gets a clear error and no API call is attempted
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestQuery_BodyMissingQueryKey(t *testing.T) {
	t.Parallel()
	// GIVEN a body that parses as JSON but lacks `query`
	testClient := gitlabtesting.NewTestClient(t)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(`{"response_format":"llm"}`),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN
	_, err := exec("-")

	// THEN the user gets a clear error and no API call is attempted
	require.Error(t, err)
	assert.Contains(t, err.Error(), "`query` object")
}

func TestQuery_Unauthorized(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 401
	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.Any(), gomock.Any()).
		Return(nil,
			&gitlab.Response{Response: &http.Response{StatusCode: http.StatusUnauthorized}},
			&gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnauthorized},
				Message:  "401 Unauthorized",
			})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// WHEN
	_, err := exec("-")

	// THEN the error maps to ExitUnauthenticated (exit code 3)
	require.Error(t, err)
	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, orbiterr.ExitUnauthenticated, exitErr.Code)
}

// TestBuildRequest_BodyResponseFormatWinsWhenFlagAbsent is a unit test for
// buildRequest directly, verifying the priority logic when formatChanged=false.
func TestBuildRequest_BodyResponseFormatWinsWhenFlagAbsent(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query":{"query_type":"traversal"},"response_format":"raw"}`)
	req, err := buildRequest(body, formatLLM, false)
	require.NoError(t, err)
	require.NotNil(t, req.ResponseFormat)
	assert.Equal(t, gitlab.OrbitResponseFormatRaw, *req.ResponseFormat, "body's response_format must win when formatChanged=false")
}

// TestBuildRequest_FlagWinsOverBody verifies that an explicit --format overrides body.
func TestBuildRequest_FlagWinsOverBody(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query":{"query_type":"traversal"},"response_format":"raw"}`)
	req, err := buildRequest(body, formatLLM, true)
	require.NoError(t, err)
	require.NotNil(t, req.ResponseFormat)
	assert.Equal(t, gitlab.OrbitResponseFormatLLM, *req.ResponseFormat, "--format must win when formatChanged=true")
}
