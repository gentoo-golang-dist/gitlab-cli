package serve

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func NewCmdServe(_ cmdutils.Factory) *cobra.Command {
	var includeCommands []string
	var excludeCommands []string

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a MCP server with stdio transport. (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Start a Model Context Protocol server to expose GitLab features
			as tools for AI assistants like Claude Code.

			The server uses stdio (standard input and output) transport for
			communication, and provides tools to:

			- Manage issues (list, create, update, close, add notes)
			- Manage merge requests (list, create, update, merge, add notes)
			- Manage projects (list, get details)
			- Manage CI/CD pipelines and jobs

			You can filter which commands are exposed as tools using the
			--include or --exclude flags. These flags operate on top-level
			commands (e.g., 'ci' matches all ci_* tools). The flags are
			mutually exclusive.

			To configure this server in Claude Code, add this code to your
			MCP settings:

			%[1]sjson
			{
			  "mcpServers": {
			    "glab": {
			      "type": "stdio",
			      "command": "glab",
			      "args": ["mcp", "serve"]
			    }
			  }
			}
			%[1]s

			To limit the available tools, add --include or --exclude flags:

			%[1]sjson
			{
			  "mcpServers": {
			    "glab": {
			      "type": "stdio",
			      "command": "glab",
			      "args": ["mcp", "serve", "--include=ci,mr,issue"]
			    }
			  }
			}
			%[1]s
		`, "```") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Start server with all tools
			$ glab mcp serve

			# Only expose CI, MR, and issue tools
			$ glab mcp serve --include=ci,mr,issue

			# Expose all tools except OpenTofu and API
			$ glab mcp serve --exclude=opentofu,api

			# Using repeated flags
			$ glab mcp serve --include=ci --include=mr --include=issue
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the root command by traversing up the parent chain
			rootCmd := cmd
			for rootCmd.Parent() != nil {
				rootCmd = rootCmd.Parent()
			}

			// Initialize the MCP server
			server := newMCPServer(rootCmd, includeCommands, excludeCommands)

			// Run the server (signal handling is done internally by server.ServeStdio)
			if err := server.Run(); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}

			return nil
		},
	}

	serveCmd.Flags().StringSliceVar(&includeCommands, "include", []string{},
		"Include only specified top-level commands (comma-separated: ci,mr,issue)")
	serveCmd.Flags().StringSliceVar(&excludeCommands, "exclude", []string{},
		"Exclude specified top-level commands (comma-separated: opentofu,api)")
	serveCmd.MarkFlagsMutuallyExclusive("include", "exclude")

	return serveCmd
}
