//go:build !integration

package query

import (
	"bytes"
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

// TestBuildRequest_PreservesAtInStringLiterals locks in the contract that
// `@` characters inside JSON string literals are NEVER preprocessed
// (no curl-style `@filename` expansion, no field-magic). These
// regression cases cover the historical confusion that surfaced as the
// "Invalid character '@'" bug report on `glab orbit remote query`:
// email addresses, Ruby `@instance_var` references, `@version`
// annotations, and `@`-prefixed query property values.
func TestBuildRequest_PreservesAtInStringLiterals(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"email address":          `{"query":{"query_type":"traversal","filter":{"email":{"eq":"user@example.com"}}}}`,
		"ruby instance variable": `{"query":{"query_type":"traversal","comment":"see @ahegyi note"}}`,
		"@version annotation":    `{"query":{"query_type":"traversal","note":"applies to @version 2"}}`,
		"@-prefixed value":       `{"query":{"query_type":"traversal","id":"@p"}}`,
		"multiple @ in string":   `{"query":{"query_type":"traversal","authors":["a@ex.com","b@ex.com"]}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req, err := buildRequest([]byte(body), formatLLM, false)
			require.NoError(t, err, "valid JSON containing @ inside string literals must parse")
			require.NotNil(t, req)

			// The Query field is json.RawMessage and must carry the
			// user's bytes through verbatim — same content, possibly
			// different whitespace, but @ characters preserved.
			assert.Contains(t, string(req.Query), "@",
				"@ characters inside JSON strings must survive buildRequest")

			// Round-trip through encoding/json to confirm the value
			// will reach the API verbatim.
			marshalled, err := json.Marshal(req)
			require.NoError(t, err)
			assert.Contains(t, string(marshalled), "@",
				"@ characters must survive marshalling for transmission")
		})
	}
}

// TestBuildRequest_InvalidJSON_AtCharacterOutsideString verifies that
// when `@` appears OUTSIDE a string literal (e.g. unrendered template
// placeholder like `@variable`), the user gets a helpful error that
// points at the real cause and at `jq` for further diagnosis — rather
// than the bare Go stdlib "invalid character '@'" message that
// historically misled users into thinking the CLI mangled the @.
func TestBuildRequest_InvalidJSON_AtCharacterOutsideString(t *testing.T) {
	t.Parallel()
	// `@variable` is unquoted — invalid JSON.
	body := []byte(`{"query": @variable}`)

	_, err := buildRequest(body, formatLLM, false)
	require.Error(t, err)

	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Contains(t, exitErr.Details, "stray '@'",
		"error message must point at the stray @ explicitly")
	assert.Contains(t, exitErr.Details, "string literal",
		"error message must mention that @ inside string literals is fine")
	assert.Contains(t, exitErr.Details, "jq",
		"error message must point users at jq for diagnosis")

	// Fang's default error handler renders err.Error() and ignores
	// ExitError.Details (see wrapJSONError godoc). Assert directly on
	// err.Error() so that any future refactor of wrapJSONError that
	// drops the bake-in still fails this test instead of silently
	// regressing the user-facing message.
	assert.Contains(t, err.Error(), "stray '@'",
		"user-facing err.Error() must contain the stray-@ hint")
	assert.Contains(t, err.Error(), "string literal",
		"user-facing err.Error() must mention string literals")
	assert.Contains(t, err.Error(), "jq",
		"user-facing err.Error() must point at jq")

	// The original *json.SyntaxError must remain reachable via the
	// error chain so callers (or future programmatic inspection) can
	// recover structured offset information. Guards against the
	// previous implementation that flattened the chain through
	// errors.New.
	var syn *json.SyntaxError
	assert.True(t, errors.As(err, &syn),
		"wrapped error must preserve the original *json.SyntaxError in the chain")
}

// TestBuildRequest_InvalidJSON_NonAtCharacter verifies that the
// generic "query body is not valid JSON" message is still used for
// non-@ syntax errors — we only special-case @ because of the
// historical bug report.
func TestBuildRequest_InvalidJSON_NonAtCharacter(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query": &foo}`)

	_, err := buildRequest(body, formatLLM, false)
	require.Error(t, err)

	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, "query body is not valid JSON", exitErr.Details,
		"non-@ syntax errors must fall back to the generic message")
}

// TestReadBody_StripsUTF8BOM exercises the defensive BOM stripping in
// readBody: some editors save files as "UTF-8 with BOM", which Go's
// encoding/json otherwise rejects with a confusing
// "invalid character 'ï'" error. `jq` accepts BOM-prefixed input;
// readBody normalises to that behaviour.
func TestReadBody_StripsUTF8BOM(t *testing.T) {
	t.Parallel()
	bomBody := append([]byte{0xEF, 0xBB, 0xBF}, []byte(stdinBody)...)
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "q.json")
	require.NoError(t, os.WriteFile(bodyPath, bomBody, 0o600))

	data, err := readBody(bodyPath, nil)
	require.NoError(t, err)

	// BOM bytes are gone and the JSON is intact.
	assert.False(t, bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}),
		"BOM must be stripped from the body")
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw),
		"after BOM stripping the body must parse as JSON")
}

// TestQuery_FromFile_WithAtInStrings is an end-to-end test confirming
// that a file containing `@` inside JSON string literals (email
// addresses, Ruby `@instance_var` references, `@version` annotations)
// is forwarded to the Orbit API verbatim — no rejection, no
// preprocessing. This is the direct regression test for the
// "Invalid character '@'" bug reported against
// `glab orbit remote query`.
func TestQuery_FromFile_WithAtInStrings(t *testing.T) {
	t.Parallel()
	const body = `{"query":{"query_type":"traversal","node":{"id":"@p","entity":"Project"},"filter":{"email":{"eq":"user@example.com"},"note":"see @ahegyi"}}}`

	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "q.json")
	require.NoError(t, os.WriteFile(bodyPath, []byte(body), 0o600))

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.AssignableToTypeOf(&gitlab.OrbitQueryRequest{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.OrbitQueryRequest, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitQueryResult, *gitlab.Response, error) {
			require.NotNil(t, opts)
			// Every @ character in the source body must reach the API.
			require.Contains(t, string(opts.Query), "user@example.com",
				"email address must survive @-handling")
			require.Contains(t, string(opts.Query), "@ahegyi",
				"@-prefixed reference inside a string must survive")
			require.Contains(t, string(opts.Query), `"@p"`,
				"@-prefixed value must survive")
			return &gitlab.OrbitQueryResult{},
				&gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
		})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	_, err := exec(bodyPath)
	require.NoError(t, err, "valid JSON with @ inside string literals must not be rejected")
}

// TestQuery_Stdin_WithAtInStrings mirrors TestQuery_FromFile_WithAtInStrings
// but exercises the stdin path, since the bug report explicitly called
// out `glab orbit remote query /tmp/q.json` AND the stdin variant.
func TestQuery_Stdin_WithAtInStrings(t *testing.T) {
	t.Parallel()
	const body = `{"query":{"query_type":"traversal","filter":{"email":{"eq":"user@example.com"}}}}`

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		Query(gomock.AssignableToTypeOf(&gitlab.OrbitQueryRequest{}), gomock.Any()).
		DoAndReturn(func(opts *gitlab.OrbitQueryRequest, _ ...gitlab.RequestOptionFunc) (*gitlab.OrbitQueryResult, *gitlab.Response, error) {
			require.Contains(t, string(opts.Query), "user@example.com")
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

	_, err := exec("-")
	require.NoError(t, err)
}
