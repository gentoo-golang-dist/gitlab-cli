package status

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

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

// NewCmd returns the `glab orbit remote status` subcommand.
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
			response as pretty-printed JSON. This is the cheapest way to confirm
			Orbit is enabled and reachable for your user, and it is the first
			step in the Orbit discovery workflow.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab orbit status
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return opts.run()
		},
	}

	cmd.Flags().StringVar(&opts.hostname, "hostname", "",
		"GitLab hostname to query. Defaults to the current repository's host or `gitlab.com`.")

	return cmd
}

func (o *options) run() error {
	client, err := o.apiClient(o.hostname)
	if err != nil {
		return err
	}

	status, _, err := client.Lab().Orbit.GetStatus()
	if err != nil {
		return orbiterr.Translate(err)
	}

	return o.io.PrintJSON(status)
}
