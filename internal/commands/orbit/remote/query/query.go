package query

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	internalAPI "gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// utf8BOM is the byte-order mark that some editors prepend to UTF-8
// files. Go's encoding/json rejects it with a misleading
// "invalid character 'ï'" message; readBody strips it so users get the
// same behaviour as `jq`, which tolerates a leading BOM.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

const (
	formatLLM = "llm"
	formatRaw = "raw"
)

type options struct {
	apiClient func(repoHost string) (*internalAPI.Client, error)
	io        *iostreams.IOStreams

	hostname string
	format   string

	source        string
	formatChanged bool
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		apiClient: f.ApiClient,
		io:        f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "query [file|-]",
		Short: `Execute a GitLab Knowledge Graph query. (EXPERIMENTAL)`,
		Long: heredoc.Docf(`
			Calls %[1]sPOST /api/v4/orbit/query%[1]s with a JSON request body and
			prints the server response verbatim. The body is read from a file
			path or from standard input when the argument is %[1]s-%[1]s or
			omitted.

			The request body must be a full Orbit query envelope:

			%[2]sjson
			{
			  "query": { "query_type": "...", ... },
			  "response_format": "llm" | "raw"
			}
			%[2]s

			%[1]s--format%[1]s overrides the body's %[1]sresponse_format%[1]s value,
			or sets it if absent. If neither the body nor %[1]s--format%[1]s
			specifies a format, %[1]sllm%[1]s is used by default. The %[1]sllm%[1]s
			format is compact GOON/TOON text intended for agents; %[1]sraw%[1]s
			returns structured JSON suitable for %[1]sjq%[1]s. The server's
			response body is written to stdout verbatim regardless of format —
			no client-side decoding or re-encoding is performed.

			The graph DSL JSON Schema is served by %[1]sglab orbit remote tools%[1]s
			and is the source of truth for the body shape. See also
			%[1]sglab orbit remote schema%[1]s for the graph ontology.

			For the full query language reference with examples, fetch the docs
			from the Knowledge Graph repository:

			%[2]sconsole
			glab api "projects/gitlab-org%%2Forbit%%2Fknowledge-graph/repository/files/docs%%2Fsource%%2Fqueries%%2Fquery_language.md?ref=main" | jq -r .content | base64 -d
			%[2]s
		`, "`", "```") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Run a query from a file
			$ glab orbit remote query ./query.json

			# Run a query from stdin
			$ cat ./query.json | glab orbit remote query -

			# Force raw output (pipeable into jq)
			$ glab orbit remote query --format raw ./query.json
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			opts.formatChanged = cmd.Flags().Changed("format")
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to query. Defaults to the current repository's host or `gitlab.com`.")
	fl.VarP(cmdutils.NewEnumValue([]string{formatLLM, formatRaw}, formatLLM, &opts.format),
		"format", "f",
		"Response format: `llm` (compact, intended for agents) or `raw` (structured JSON).")

	return cmd
}

func (o *options) complete(args []string) {
	o.source = "-"
	if len(args) == 1 {
		o.source = args[0]
	}
}

func (o *options) run(ctx context.Context) error {
	bodyBytes, err := readBody(o.source, o.io.In)
	if err != nil {
		return err
	}

	req, err := buildRequest(bodyBytes, o.format, o.formatChanged)
	if err != nil {
		return err
	}

	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	// We deliberately bypass `client.Lab().Orbit.Query` and instead
	// stream the raw response body to stdout. The typed SDK helper
	// decodes the response via `json.NewDecoder`, which is correct
	// only for `response_format=raw` (JSON envelope) and fails with
	// `invalid character '@' looking for beginning of value` for
	// `response_format=llm`, where the server returns GOON/TOON text
	// (Content-Type: text/plain) starting with `@header`.
	//
	// Passing `*bytes.Buffer` (an `io.Writer`) to `client.Do` makes
	// the SDK copy the body verbatim instead of decoding it, so both
	// formats round-trip unmodified.
	httpReq, err := client.Lab().NewRequest(
		http.MethodPost,
		"orbit/query",
		req,
		[]gitlab.RequestOptionFunc{gitlab.WithContext(ctx)},
	)
	if err != nil {
		return fmt.Errorf("building Orbit query request: %w", err)
	}

	var respBody bytes.Buffer
	if _, err := client.Lab().Do(httpReq, &respBody); err != nil {
		return orbiterr.Translate(err)
	}

	_, err = o.io.StdOut.Write(respBody.Bytes())
	return err
}

// readBody reads the request body from a file path or from `stdin`
// when the source is `-`.
func readBody(source string, stdin io.ReadCloser) ([]byte, error) {
	var r io.ReadCloser
	if source == "-" {
		r = stdin
	} else {
		f, err := os.Open(source)
		if err != nil {
			return nil, fmt.Errorf("opening query body: %w", err)
		}
		r = f
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading query body: %w", err)
	}
	// Strip a leading UTF-8 BOM if present. Some editors add it to
	// files saved as "UTF-8 with BOM"; Go's JSON parser rejects it
	// even though the JSON is otherwise valid (and `jq` accepts it).
	data = bytes.TrimPrefix(data, utf8BOM)
	if len(data) == 0 {
		return nil, errors.New("query body is empty")
	}
	return data, nil
}

// wrapJSONError returns a context-rich `*cmdutils.ExitError` for a
// JSON parse failure. The default Go error ("invalid character '@'
// looking for beginning of value") is technically accurate but
// historically confused users into thinking the CLI mis-handles `@`
// inside JSON string literals — when in fact `@` outside a string is
// just invalid JSON. This helper points the user at the real cause
// and at `jq`, which they can use to locate the offending byte.
//
// The body is *not* preprocessed before JSON parsing: bytes inside
// JSON string literals (including `@`) are forwarded verbatim.
//
// The detail text is baked into the wrapped error's message because
// Fang's default error handler renders `err.Error()` and ignores
// `ExitError.Details`.
func wrapJSONError(body []byte, err error) *cmdutils.ExitError {
	const base = "query body is not valid JSON"
	details := base
	// We rely solely on `body[Offset-1] == '@'` to detect the
	// stray-`@` case. We deliberately do NOT substring-match on the
	// stdlib's "looking for beginning of value" wording: that string
	// is not part of Go's API contract and could be reworded in a
	// future release, which would silently disable this special case.
	// The byte check is sufficient on its own — `@` inside a JSON
	// string literal never produces a SyntaxError, so any
	// SyntaxError whose offending byte is `@` is by definition the
	// stray-`@` case we want to flag.
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		if ch, ok := byteAtOffset(body, syn.Offset); ok && ch == '@' {
			details = fmt.Sprintf(
				"%s: stray %q outside a string literal at byte %d (1-indexed). "+
					"`@` is allowed inside JSON string values (e.g. \"user@example.com\"); "+
					"if your file looks correct, validate it with `jq . <file>` to find the real offset",
				base, ch, syn.Offset)
		}
	}
	// Wrap with %w so the original *json.SyntaxError remains
	// reachable via errors.As / errors.Unwrap. Fang renders
	// err.Error() (which contains the full hint baked in via
	// fmt.Errorf), so the user-facing message is unchanged.
	return cmdutils.WrapError(fmt.Errorf("%s: %w", details, err), details)
}

// byteAtOffset returns the byte at the offending position reported by
// `json.SyntaxError.Offset`. `Offset` is the 1-indexed byte position
// just past the offending character (i.e. the number of bytes read
// before the parser failed), so the offending byte itself lives at
// `Offset-1`.
func byteAtOffset(body []byte, offset int64) (byte, bool) {
	idx := offset - 1
	if idx < 0 || idx >= int64(len(body)) {
		return 0, false
	}
	return body[idx], true
}

// buildRequest parses the user-supplied body into an
// *gitlab.OrbitQueryRequest. The user's query is preserved
// verbatim; response_format priority is:
//  1. --format flag (when explicitly passed by the user)
//  2. body's response_format field
//  3. "llm" fallback default
//
// The body is parsed strictly as JSON: there is no curl-style
// `@filename` expansion or any other preprocessing. Bytes inside JSON
// string literals — including `@` (e.g. email addresses like
// `user@example.com`, Ruby `@instance_var` references, or `@version`
// annotations) — are forwarded to the API verbatim.
func buildRequest(body []byte, format string, formatChanged bool) (*gitlab.OrbitQueryRequest, error) {
	var raw struct {
		Query          json.RawMessage `json:"query"`
		ResponseFormat *string         `json:"response_format,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, wrapJSONError(body, err)
	}
	if len(raw.Query) == 0 {
		return nil, errors.New("query body must contain a top-level `query` object")
	}

	var chosen gitlab.OrbitResponseFormatValue
	switch {
	case formatChanged:
		chosen = gitlab.OrbitResponseFormatValue(format)
	case raw.ResponseFormat != nil:
		chosen = gitlab.OrbitResponseFormatValue(*raw.ResponseFormat)
	default:
		chosen = formatLLM
	}
	return &gitlab.OrbitQueryRequest{
		Query:          raw.Query,
		ResponseFormat: &chosen,
	}, nil
}
