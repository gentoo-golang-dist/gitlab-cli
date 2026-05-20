package dsl

import (
	"context"
	"fmt"

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
		Use:   "dsl",
		Short: `Show the GitLab Knowledge Graph query DSL JSON Schema. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Calls `+"`GET /api/v4/orbit/schema/dsl`"+` and prints the query DSL
			JSON Schema. This is the source of truth for the query body shape.
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			$ glab orbit remote dsl
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

	// Pass nil opts: response_format defaults to "raw" (JSON Schema)
	// server-side. GetDsl returns the body verbatim as a string.
	dsl, _, err := client.Lab().Orbit.GetDsl(nil, gitlab.WithContext(ctx))
	if err != nil {
		return orbiterr.Translate(err)
	}

	// Trailing newline so the shell prompt doesn't glue to the last byte
	// of the body when the server response itself doesn't end in one.
	_, err = fmt.Fprintln(o.io.StdOut, dsl)
	return err
}
