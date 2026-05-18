//go:build !integration

package query

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

const stdinBody = `{"query":{"query_type":"traversal","node":{"id":"p","entity":"Project"},"limit":1}}`

// orbitServer is a thin httptest.Server wrapper used by every test in
// this file. It captures the parsed request body so individual tests
// can assert on the query and chosen response_format without
// re-implementing the boilerplate.
type orbitServer struct {
	server      *httptest.Server
	requestBody gitlab.OrbitQueryRequest
	rawBody     []byte
	called      bool
}

// newOrbitServer returns an httptest.Server that serves a single
// `POST /api/v4/orbit/query` request. The handler responds with
// `respBody` (no Content-Type override), unless `respFn` is non-nil,
// in which case the test takes full control of the response.
//
// We deliberately use a real HTTP server rather than the
// gitlabtesting.MockOrbit mock: production now bypasses
// `OrbitService.Query` (which decodes via json.NewDecoder and fails
// for non-JSON `llm` responses) and streams the response body
// verbatim, so the tests must exercise the wire format that the
// command actually consumes.
func newOrbitServer(t *testing.T, respBody string, respFn func(w http.ResponseWriter, r *http.Request)) *orbitServer {
	t.Helper()
	s := &orbitServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.called = true
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v4/orbit/query", r.URL.Path)

		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		s.rawBody = raw
		require.NoError(t, json.Unmarshal(raw, &s.requestBody))

		if respFn != nil {
			respFn(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(s.server.Close)
	return s
}

// apiClientOption builds the *api.Client wired to point at this
// orbitServer, ready to pass to cmdtest.WithApiClient.
func (s *orbitServer) apiClientOption(t *testing.T) cmdtest.FactoryOption {
	t.Helper()
	return cmdtest.WithApiClient(
		cmdtest.NewTestApiClient(t, nil, "", "", api.WithBaseURL(s.server.URL+"/api/v4/")),
	)
}

func TestQuery_Stdin_DefaultsToLLM(t *testing.T) {
	t.Parallel()
	// GIVEN a body on stdin and no --format flag
	respBody := "@header\nquery_type:traversal\n@nodes\nProject(1):\n278964 name=GitLab\n"
	srv := newOrbitServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		// The server uses Content-Type: text/plain for `llm` —
		// reproduce that here to lock in the contract that the CLI
		// passes non-JSON bodies through verbatim.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, respBody)
	})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		srv.apiClientOption(t),
	)

	// WHEN `glab orbit remote query -` runs (stdin)
	out, err := exec("-")
	require.NoError(t, err)

	// THEN the body's chosen response_format is `llm` (default)
	require.True(t, srv.called)
	require.NotNil(t, srv.requestBody.ResponseFormat)
	assert.Equal(t, gitlab.OrbitResponseFormatLLM, *srv.requestBody.ResponseFormat)

	// AND the user-supplied query is forwarded verbatim
	var got map[string]any
	require.NoError(t, json.Unmarshal(srv.requestBody.Query, &got))
	assert.Equal(t, "traversal", got["query_type"])

	// AND the non-JSON GOON body is printed verbatim to stdout —
	// this is the regression test for the historical
	// "Invalid character '@' looking for beginning of value" bug
	// where the SDK tried to json.Decode a `@header...` body.
	assert.Equal(t, respBody, out.OutBuf.String())
}

func TestQuery_FlagOverridesBodyResponseFormat(t *testing.T) {
	t.Parallel()
	// GIVEN the body sets response_format=raw but --format=llm is passed
	body := `{"query":{"query_type":"neighbors","node_ids":[1]},"response_format":"raw"}`
	srv := newOrbitServer(t, `{"result":null}`, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(body),
		srv.apiClientOption(t),
	)

	_, err := exec("--format llm -")
	require.NoError(t, err)

	require.True(t, srv.called)
	require.NotNil(t, srv.requestBody.ResponseFormat)
	assert.Equal(t, gitlab.OrbitResponseFormatLLM, *srv.requestBody.ResponseFormat,
		"--format must override the body's response_format")
}

func TestQuery_BodyFormatHonoredWhenNoFlag(t *testing.T) {
	t.Parallel()
	// GIVEN the body sets response_format=raw and --format is NOT passed
	body := `{"query":{"query_type":"neighbors","node_ids":[1]},"response_format":"raw"}`
	srv := newOrbitServer(t, `{"result":null}`, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(body),
		srv.apiClientOption(t),
	)

	_, err := exec("-")
	require.NoError(t, err)

	// THEN the body's response_format wins (previously the cobra
	// default of "llm" silently overrode it).
	require.True(t, srv.called)
	require.NotNil(t, srv.requestBody.ResponseFormat)
	assert.Equal(t, gitlab.OrbitResponseFormatRaw, *srv.requestBody.ResponseFormat,
		"body's response_format must win when --format is not passed")
}

func TestQuery_FromFile(t *testing.T) {
	t.Parallel()
	// GIVEN the body lives in a file
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "q.json")
	require.NoError(t, os.WriteFile(bodyPath, []byte(stdinBody), 0o600))

	srv := newOrbitServer(t, `{"result":null}`, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		srv.apiClientOption(t),
	)

	// WHEN `glab orbit remote query <file>` runs
	_, err := exec(bodyPath)
	require.NoError(t, err)
	assert.True(t, srv.called)
}

func TestQuery_InvalidFormatFlag(t *testing.T) {
	t.Parallel()
	// GIVEN a body on stdin and an invalid --format value.
	// NewEnumValue rejects unknown values at flag parsing time,
	// before RunE runs — so no HTTP call should reach the server.
	srv := newOrbitServer(t, "", nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		srv.apiClientOption(t),
	)

	_, err := exec("--format yaml -")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
	assert.False(t, srv.called, "flag-validation errors must not hit the API")
}

func TestQuery_EmptyBody(t *testing.T) {
	t.Parallel()
	srv := newOrbitServer(t, "", nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(""),
		srv.apiClientOption(t),
	)

	_, err := exec("-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.False(t, srv.called, "empty-body errors must not hit the API")
}

func TestQuery_BodyMissingQueryKey(t *testing.T) {
	t.Parallel()
	srv := newOrbitServer(t, "", nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(`{"response_format":"llm"}`),
		srv.apiClientOption(t),
	)

	_, err := exec("-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "`query` object")
	assert.False(t, srv.called, "validation errors must not hit the API")
}

func TestQuery_Unauthorized(t *testing.T) {
	t.Parallel()
	// GIVEN the API returns 401
	srv := newOrbitServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"401 Unauthorized"}`)
	})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		srv.apiClientOption(t),
	)

	_, err := exec("-")
	// THEN the error maps to ExitUnauthenticated (exit code 3)
	require.Error(t, err)
	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, orbiterr.ExitUnauthenticated, exitErr.Code)
}

// TestQuery_LLMResponseStreamedVerbatim is the direct regression test
// for the bug that motivated this fix: when the server returns
// `response_format=llm` (GOON/TOON text starting with `@header`), the
// previous implementation passed the body through
// `OrbitService.Query` → `json.NewDecoder(...).Decode(...)`, which
// failed with the misleading
// "Invalid character '@' looking for beginning of value" error on
// every llm query, regardless of correctness.
//
// We assert here that the bytes the server writes are the bytes the
// CLI prints, byte-for-byte. The body deliberately contains `@`
// characters so that any future refactor that re-introduces a JSON
// decode of the response body fails this test loudly.
func TestQuery_LLMResponseStreamedVerbatim(t *testing.T) {
	t.Parallel()
	// Sample GOON output captured from a real `response_format=llm`
	// response — every line starts with content that would break a
	// JSON decoder.
	respBody := strings.Join([]string{
		"@header",
		"query_type:traversal",
		"goon_version:1.0.0",
		"nodes:1",
		"edges:0",
		"@nodes",
		"Project(1):",
		"278964 name=GitLab",
		"@edges",
		"",
	}, "\n")

	srv := newOrbitServer(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, respBody)
	})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		srv.apiClientOption(t),
	)

	out, err := exec("--format llm -")
	require.NoError(t, err,
		"`response_format=llm` body must be streamed verbatim — "+
			"no JSON decode of the response is allowed")
	assert.Equal(t, respBody, out.OutBuf.String(),
		"every byte of the GOON response must be written to stdout unchanged")
}

// TestQuery_RawResponseStreamedVerbatim locks in that raw (JSON)
// responses also pass through unchanged. We rely on a byte-exact
// comparison rather than re-decoding so that any future client-side
// re-marshalling (which would reorder keys or change whitespace) is
// caught.
func TestQuery_RawResponseStreamedVerbatim(t *testing.T) {
	t.Parallel()
	respBody := `{"result":{"format_version":"2.0.0","query_type":"traversal","nodes":[{"type":"Project","id":"278964","name":"GitLab"}],"edges":[]},"query_type":"traversal","row_count":1}`
	srv := newOrbitServer(t, respBody, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(stdinBody),
		srv.apiClientOption(t),
	)

	out, err := exec("--format raw -")
	require.NoError(t, err)
	assert.Equal(t, respBody, out.OutBuf.String(),
		"raw JSON response must reach stdout byte-for-byte unchanged")
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

	srv := newOrbitServer(t, `{"result":null}`, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		srv.apiClientOption(t),
	)

	_, err := exec(bodyPath)
	require.NoError(t, err, "valid JSON with @ inside string literals must not be rejected")

	// Every @ character in the source body must reach the API.
	require.True(t, srv.called)
	assert.Contains(t, string(srv.requestBody.Query), "user@example.com",
		"email address must survive @-handling")
	assert.Contains(t, string(srv.requestBody.Query), "@ahegyi",
		"@-prefixed reference inside a string must survive")
	assert.Contains(t, string(srv.requestBody.Query), `"@p"`,
		"@-prefixed value must survive")
}

// TestQuery_Stdin_WithAtInStrings mirrors TestQuery_FromFile_WithAtInStrings
// but exercises the stdin path, since the bug report explicitly called
// out `glab orbit remote query /tmp/q.json` AND the stdin variant.
func TestQuery_Stdin_WithAtInStrings(t *testing.T) {
	t.Parallel()
	const body = `{"query":{"query_type":"traversal","filter":{"email":{"eq":"user@example.com"}}}}`

	srv := newOrbitServer(t, `{"result":null}`, nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithStdin(body),
		srv.apiClientOption(t),
	)

	_, err := exec("-")
	require.NoError(t, err)

	require.True(t, srv.called)
	assert.Contains(t, string(srv.requestBody.Query), "user@example.com")
}
