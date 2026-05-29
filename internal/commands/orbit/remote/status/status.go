package status

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
		Use:   "status",
		Short: `Show GitLab Knowledge Graph cluster health. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Calls `+"`GET /api/v4/orbit/status`"+` and prints the cluster health
			response as pretty-printed JSON. Use this command to confirm Orbit
			is enabled and reachable for your user. It is the first step in
			the Orbit discovery workflow.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab orbit remote status
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

	cmdutils.AddJQFlag(cmd, f.IO())
	return cmd
}

func (o *options) run(ctx context.Context) error {
	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	status, _, err := client.Lab().Orbit.GetStatus(nil, gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(status)
}
