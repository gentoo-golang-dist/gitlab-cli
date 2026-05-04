package graphstatus

import (
	"errors"
	"fmt"
	"net/http"

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

	hostname    string
	namespaceID int64
	projectID   int64
	fullPath    string
	format      string
}

// NewCmd returns the `glab orbit remote graph-status` subcommand.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		apiClient: f.ApiClient,
		io:        f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "graph-status",
		Short: `Show indexing progress for a namespace or project. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Calls `+"`GET /api/v4/orbit/graph_status`"+` and prints the
			indexing-progress response as pretty-printed JSON. The response
			carries project counts, per-domain entity counts, and the overall
			indexing run state for the requested scope.

			Exactly one of `+"`--namespace-id`"+`, `+"`--project-id`"+`, or
			`+"`--full-path`"+` is required. `+"`--full-path`"+` accepts the
			full path of a project or group, e.g. `+"`gitlab-org/gitlab`"+`.

			Unlike `+"`glab orbit remote query`"+`, this endpoint defaults to
			the `+"`raw`"+` response format. Use `+"`--format llm`"+` for the
			compact, agent-friendly output.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Look up indexing progress by full path
			$ glab orbit remote graph-status --full-path gitlab-org/gitlab

			# Or by numeric ID
			$ glab orbit remote graph-status --project-id 278964
			$ glab orbit remote graph-status --namespace-id 9970

			# Compact output for agents
			$ glab orbit remote graph-status --full-path gitlab-org/gitlab --format llm
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to query. Defaults to the current repository's host or `gitlab.com`.")
	fl.Int64Var(&opts.namespaceID, "namespace-id", 0,
		"Namespace (group) ID to inspect. Mutually exclusive with `--project-id` and `--full-path`.")
	fl.Int64Var(&opts.projectID, "project-id", 0,
		"Project ID to inspect. Mutually exclusive with `--namespace-id` and `--full-path`.")
	fl.StringVar(&opts.fullPath, "full-path", "",
		"Full path of a project or group, e.g. `gitlab-org/gitlab`. Mutually exclusive with the ID flags.")
	fl.StringVarP(&opts.format, "format", "f", "",
		"Response format: `raw` (structured JSON) or `llm` (compact, agent-friendly). Default: `raw`.")

	cmd.MarkFlagsMutuallyExclusive("namespace-id", "project-id", "full-path")
	cmd.MarkFlagsOneRequired("namespace-id", "project-id", "full-path")

	return cmd
}

func (o *options) validate() error {
	if o.format != "" && o.format != formatLLM && o.format != formatRaw {
		return cmdutils.FlagError{Err: fmt.Errorf("--format must be %q or %q, got %q",
			formatLLM, formatRaw, o.format)}
	}
	return nil
}

func (o *options) run() error {
	apiOpts := &gitlab.GetGraphStatusOptions{}

	if o.namespaceID != 0 {
		nid := o.namespaceID
		apiOpts.NamespaceID = &nid
	}
	if o.projectID != 0 {
		pid := o.projectID
		apiOpts.ProjectID = &pid
	}
	if o.fullPath != "" {
		fp := o.fullPath
		apiOpts.FullPath = &fp
	}
	if o.format != "" {
		f := o.format
		apiOpts.ResponseFormat = &f
	}

	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	status, resp, err := client.Lab().Orbit.GetGraphStatus(apiOpts)
	if err != nil {
		// `graph_status` can return 503 when the underlying GKG service
		// is unavailable. The shared translator does not have a mapping
		// for 503, so surface it as a clear, descriptive generic-exit
		// error before falling through to the standard taxonomy.
		if resp != nil && resp.StatusCode == http.StatusServiceUnavailable {
			return cmdutils.WrapError(
				errors.New("Knowledge Graph service unavailable"),
				"The Orbit API returned HTTP 503. The underlying GKG service is\n"+
					"currently unreachable; retry shortly or check the GitLab status\n"+
					"page.",
			)
		}
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(status)
}
