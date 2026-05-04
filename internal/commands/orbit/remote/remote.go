package remote

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	graphStatusCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote/graphstatus"
	queryCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote/query"
	schemaCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote/schema"
	statusCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote/status"
	toolsCmd "gitlab.com/gitlab-org/cli/internal/commands/orbit/remote/tools"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

// NewCmd returns the `glab orbit remote` parent command and
// wires its API subcommands.
func NewCmd(f cmdutils.Factory) *cobra.Command {
	remoteCmd := &cobra.Command{
		Use:     "remote <command> [flags]",
		Aliases: []string{"r"},
		Short:   `Interact with the remote GitLab Knowledge Graph. (EXPERIMENTAL)`,
		Long: heredoc.Doc(`
			Query the remote GitLab Knowledge Graph (product name: Orbit)
			directly from the CLI. All endpoints are user-scoped (not
			project-scoped) and gated behind the `+"`knowledge_graph`"+`
			feature flag.

			Always start with the discovery dance — these calls are cheap and
			return the authoritative ontology and query DSL schema:

			`+"```shell"+`
			glab orbit remote status                         # is the service up?
			glab orbit remote schema                         # what entities and edges exist?
			glab orbit remote schema MergeRequest Project    # drill into specific nodes
			glab orbit remote tools                          # full DSL JSON Schema
			`+"```"+`

			Once you know the shape of the graph, run a query:

			`+"```shell"+`
			glab orbit remote query path/to/query.json
			cat query.json | glab orbit remote query -
			`+"```"+`

			Inspect indexing progress for a namespace or project:

			`+"```shell"+`
			glab orbit remote graph-status --full-path gitlab-org/gitlab
			`+"```"+`

			Exit codes:

			- 1 — generic error
			- 2 — Orbit endpoint unavailable (HTTP 404, e.g. feature flag off)
			- 3 — not authenticated (HTTP 401)
			- 4 — access denied (HTTP 403, e.g. no Knowledge Graph enabled
			  namespaces)
			- 5 — rate limited (HTTP 429)
		`) + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Discovery workflow (always start here)
			$ glab orbit remote status
			$ glab orbit remote schema
			$ glab orbit remote schema User Project MergeRequest
			$ glab orbit remote tools

			# Run a query from a file
			$ glab orbit remote query ./query.json

			# Run a query from stdin (raw output for jq pipelines)
			$ echo '{"query":{"query_type":"traversal","node":{"id":"p","entity":"Project"},"limit":5}}' \
			    | glab orbit remote query --format raw -

			# Inspect indexing progress
			$ glab orbit remote graph-status --full-path gitlab-org/gitlab
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
	}

	remoteCmd.AddCommand(statusCmd.NewCmd(f))
	remoteCmd.AddCommand(schemaCmd.NewCmd(f))
	remoteCmd.AddCommand(toolsCmd.NewCmd(f))
	remoteCmd.AddCommand(queryCmd.NewCmd(f))
	remoteCmd.AddCommand(graphStatusCmd.NewCmd(f))

	return remoteCmd
}
