package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
			prints the response as pretty-printed JSON. The body is read from
			a file path or from standard input when the argument is %[1]s-%[1]s
			or omitted.

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
			format is compact and intended for agents. Use %[1]s--format raw%[1]s
			when piping into %[1]sjq%[1]s.

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

	result, _, err := client.Lab().Orbit.Query(req, gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(result)
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
	if len(data) == 0 {
		return nil, errors.New("query body is empty")
	}
	return data, nil
}

// buildRequest parses the user-supplied body into an
// *gitlab.OrbitQueryRequest. The user's query is preserved
// verbatim; response_format priority is:
//  1. --format flag (when explicitly passed by the user)
//  2. body's response_format field
//  3. "llm" fallback default
func buildRequest(body []byte, format string, formatChanged bool) (*gitlab.OrbitQueryRequest, error) {
	var raw struct {
		Query          json.RawMessage `json:"query"`
		ResponseFormat *string         `json:"response_format,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, cmdutils.WrapError(err, "query body is not valid JSON")
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
