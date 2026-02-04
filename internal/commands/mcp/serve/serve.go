package serve

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io        *iostreams.IOStreams
	transport string
	port      string
	address   string
}

func NewCmdServe(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io: f.IO(),
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a MCP server. (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Start a Model Context Protocol server to expose GitLab features
			as tools for AI assistants and agents.

			The server supports two transport modes:

			- stdio (default): For local integration with MCP clients.
			  Process lifecycle is managed by the client.

			- streamable-http: For network access, multiple concurrent clients,
			  or deployment scenarios. Server runs independently and must be
			  manually started and stopped.

			The server provides tools to:

			- Manage issues (list, create, update, close, add notes)
			- Manage merge requests (list, create, update, merge, add notes)
			- Manage projects (list, get details)
			- Manage CI/CD pipelines and jobs

			For stdio transport, configure your MCP client with:

			- Command: glab
			- Args: ["mcp", "serve"]

			For streamable-http transport:

			%[1]sshell
			# Start the server
			glab mcp serve --transport streamable-http --port 8080

			# Server will run until stopped with Ctrl+C
			# Connect MCP clients to http://localhost:8080
			%[1]s

			Use cases for HTTP transport:
			
			- Remote access from another machine
			- Multiple concurrent client connections
			- Custom MCP client integrations
			- Long-running independent server
		`, "```") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Start with stdio transport (default)
			$ glab mcp serve

			# Start with HTTP transport
			$ glab mcp serve --transport streamable-http --port 8080

			# Start HTTP transport on specific address
			$ glab mcp serve --transport streamable-http --port 8080 --address 127.0.0.1
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}

			// Get the root command by traversing up the parent chain
			rootCmd := cmd
			for rootCmd.Parent() != nil {
				rootCmd = rootCmd.Parent()
			}

			// Initialize the MCP server
			server := newMCPServer(rootCmd)

			// Run the server with context
			if err := server.Run(cmd.Context(), opts); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}

			return nil
		},
	}

	fl := serveCmd.Flags()
	fl.StringVar(&opts.transport, "transport", "stdio", "Transport mode: stdio or streamable-http")
	fl.StringVar(&opts.port, "port", "8080", "Port for HTTP server (only used with streamable-http transport)")
	fl.StringVar(&opts.address, "address", "localhost", "Address to bind HTTP server (only used with streamable-http transport)")

	return serveCmd
}

func (o *options) validate() error {
	if o.transport != "stdio" && o.transport != "streamable-http" {
		return fmt.Errorf("invalid transport: %s (must be 'stdio' or 'streamable-http')", o.transport)
	}

	if o.transport == "streamable-http" {
		if port, err := strconv.Atoi(o.port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port: %s (must be a number between 1 and 65535)", o.port)
		}
	}

	return nil
}
