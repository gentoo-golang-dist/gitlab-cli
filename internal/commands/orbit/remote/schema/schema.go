package schema

import (
	"context"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams

	hostname string
	expand   []string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		apiClient: f.ApiClient,
		io:        f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "schema [node...]",
		Short: `Show the GitLab Knowledge Graph ontology. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Calls `+"`GET /api/v4/orbit/schema`"+` and prints the response as
			pretty-printed JSON. The response carries the authoritative graph
			ontology, including domains, nodes, and edges. It is the source
			of truth when writing queries.

			Positional arguments are passed through as the `+"`expand`"+` query
			parameter (comma-joined). Listed nodes are returned with their full
			properties, style, and incoming and outgoing edge lists. Unlisted nodes
			remain summary-only in the same response.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Show the full schema (compact, no node detail)
			$ glab orbit remote schema

			# Show details for specific nodes
			$ glab orbit remote schema User Project MergeRequest
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to query. Defaults to the current repository's host or `gitlab.com`.")

	cmdutils.AddJQFlag(cmd, f.IO())
	return cmd
}

func (o *options) complete(args []string) {
	o.expand = args
}

func (o *options) run(ctx context.Context) error {
	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	var schemaOpts *gitlab.GetOrbitSchemaOptions
	if len(o.expand) > 0 {
		schemaOpts = &gitlab.GetOrbitSchemaOptions{Expand: &o.expand}
	}

	schema, _, err := client.Lab().Orbit.GetSchema(schemaOpts, gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(schema)
}
