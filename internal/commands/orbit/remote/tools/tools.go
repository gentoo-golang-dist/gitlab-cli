package tools

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
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		apiClient: f.ApiClient,
		io:        f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "tools",
		Short: `Show the GitLab Knowledge Graph MCP tool manifest. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Calls `+"`GET /api/v4/orbit/tools`"+` and prints the MCP tool manifest
			as pretty-printed JSON. The manifest carries the authoritative JSON
			Schema for the query DSL inside the `+"`parameters`"+` field of the
			`+"`query_graph`"+` tool. It is the source of truth for the query
			body shape.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab orbit remote tools
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to query. Defaults to the current repository's host or `gitlab.com`.")

	return cmd
}

func (o *options) run(ctx context.Context) error {
	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	tools, _, err := client.Lab().Orbit.GetTools(gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(tools.Tools)
}
