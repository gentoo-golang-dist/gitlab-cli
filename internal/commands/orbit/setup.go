package orbit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

const (
	skillRepo    = "gitlab-org%2Forbit%2Fknowledge-graph"
	skillRef     = "main"
	orbitMCPName = "gitlab-orbit"
)

var skillFiles = []string{
	"skills/orbit/SKILL.md",
	"skills/orbit/references/query_language.md",
	"skills/orbit/references/recipes.md",
	"skills/orbit/references/troubleshooting.md",
	"skills/orbit/scripts/orbit-query",
}

type setupOptions struct {
	agent     string
	skillOnly bool
	mcpOnly   bool
	dryRun    bool
}

func newSetupCmd(f cmdutils.Factory) *cobra.Command {
	opts := &setupOptions{}

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install the Orbit skill and MCP config for your AI coding agent",
		Long: heredoc.Doc(`
			Detects your AI coding agent, installs the Orbit skill context files,
			and writes MCP configuration so external agents can query the
			GitLab Knowledge Graph (Orbit).

			Supported agents: Claude Code, OpenCode, Cursor, Codex, Gemini CLI, Duo CLI.
		`),
		Example: heredoc.Doc(`
			# Auto-detect agent and set up everything
			glab orbit setup

			# Override detected agent
			glab orbit setup --agent=cursor

			# Install skill only, skip MCP config
			glab orbit setup --skill-only

			# Preview without writing anything
			glab orbit setup --dry-run
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.agent == "" {
				opts.agent = detectAgent()
			}

			streams := f.IO()
			cs := streams.Color()

			hostname := f.DefaultHostname()
			apiClient, err := f.ApiClient(hostname)
			if err != nil {
				return fmt.Errorf("authentication required: run 'glab auth login' first: %w", err)
			}
			httpClient := apiClient.HTTPClient()
			baseURL := apiClient.BaseURL()

			skillDir := skillDirFor(opts.agent)
			mcpConfig := mcpConfigFor(opts.agent)

			if opts.agent == "unknown" {
				instanceURL := instanceURLFromBase(baseURL)
				fmt.Fprintf(streams.StdOut, "No supported agent detected. Add this to your MCP config manually:\n\n")
				printManualConfig(streams.StdOut, instanceURL)
				return nil
			}

			fmt.Fprintf(streams.StdOut, "%s Detected agent: %s\n", cs.GreenCheck(), opts.agent)

			if opts.dryRun {
				fmt.Fprintf(streams.StdOut, "%s Dry run - no changes written\n", cs.WarnIcon())
				if skillDir != "" {
					fmt.Fprintf(streams.StdOut, "  Would install skill to: %s\n", skillDir)
				}
				if mcpConfig != "" {
					fmt.Fprintf(streams.StdOut, "  Would write MCP config to: %s\n", mcpConfig)
				}
				return nil
			}

			if !opts.mcpOnly && skillDir != "" {
				if err := fetchSkill(httpClient, baseURL, skillDir); err != nil {
					return err
				}
				fmt.Fprintf(streams.StdOut, "%s Skill installed: %s\n", cs.GreenCheck(), skillDir)
			} else if !opts.mcpOnly && skillDir == "" {
				fmt.Fprintf(streams.StdOut, "%s No skill convention for %s - skipping\n", cs.WarnIcon(), opts.agent)
			}

			if !opts.skillOnly && mcpConfig != "" {
				instanceURL := instanceURLFromBase(baseURL)
				if err := writeMCPConfig(opts.agent, mcpConfig, instanceURL); err != nil {
					return err
				}
				fmt.Fprintf(streams.StdOut, "%s MCP config written: %s\n", cs.GreenCheck(), mcpConfig)
			} else if !opts.skillOnly && mcpConfig == "" {
				fmt.Fprintf(streams.StdOut, "%s No MCP config path for %s - skipping\n", cs.WarnIcon(), opts.agent)
			}

			fmt.Fprintln(streams.StdOut)
			verifyOrbitAPI(httpClient, baseURL, streams)
			fmt.Fprintf(streams.StdOut, "\nOrbit is ready. Ask your agent: \"Check the Orbit API status\"\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.agent, "agent", "", "Override detected agent (claude-code, opencode, cursor, codex, gemini, duo-cli)")
	cmd.Flags().BoolVar(&opts.skillOnly, "skill-only", false, "Install skill only, skip MCP config")
	cmd.Flags().BoolVar(&opts.mcpOnly, "mcp-only", false, "Write MCP config only, skip skill install")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print what would happen without writing anything")

	return cmd
}

func detectAgent() string {
	home, _ := os.UserHomeDir()
	switch {
	case dirExists(filepath.Join(home, ".claude")):
		return "claude-code"
	case dirExists(filepath.Join(home, ".cursor")):
		return "cursor"
	case dirExists(filepath.Join(home, ".config", "opencode")):
		return "opencode"
	case dirExists(filepath.Join(home, ".codex")):
		return "codex"
	case dirExists(filepath.Join(home, ".gemini")):
		return "gemini"
	case dirExists(filepath.Join(home, ".gitlab", "duo")):
		return "duo-cli"
	default:
		return "unknown"
	}
}

func skillDirFor(agent string) string {
	home, _ := os.UserHomeDir()
	switch agent {
	case "claude-code":
		return filepath.Join(home, ".claude", "skills", "orbit")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "skills", "orbit")
	case "duo-cli":
		return filepath.Join(home, ".gitlab", "duo", "skills", "orbit")
	default:
		return ""
	}
}

func mcpConfigFor(agent string) string {
	home, _ := os.UserHomeDir()
	switch agent {
	case "claude-code":
		return filepath.Join(home, ".claude", "settings.json")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "opencode.json")
	case "cursor":
		return filepath.Join(home, ".cursor", "mcp.json")
	case "codex":
		return filepath.Join(home, ".codex", "config.json")
	case "gemini":
		return filepath.Join(home, ".gemini", "settings.json")
	default:
		return ""
	}
}

func fetchSkill(client *http.Client, baseURL, skillDir string) error {
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755); err != nil {
		return err
	}

	for _, file := range skillFiles {
		encoded := url.PathEscape(file)
		rawURL := fmt.Sprintf("%sprojects/%s/repository/files/%s/raw?ref=%s", baseURL, skillRepo, encoded, skillRef)

		resp, err := client.Get(rawURL) //nolint:noctx
		if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", file, err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to fetch %s: HTTP %d - check glab auth and network", file, resp.StatusCode)
		}

		relPath := strings.TrimPrefix(file, "skills/orbit/")
		dest := filepath.Join(skillDir, relPath)

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		if err := os.WriteFile(dest, data, 0644); err != nil { //nolint:gosec
			return fmt.Errorf("failed to write %s: %w", dest, err)
		}

		if strings.HasSuffix(file, "orbit-query") {
			_ = os.Chmod(dest, 0755) //nolint:gosec
		}
	}

	return nil
}

func writeMCPConfig(agent, configFile, instanceURL string) error {
	mcpURL := instanceURL + "/api/v4/orbit/mcp"

	data, _ := os.ReadFile(configFile)
	config := map[string]interface{}{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &config)
	}

	switch agent {
	case "claude-code":
		servers, _ := config["mcpServers"].(map[string]interface{})
		if servers == nil {
			servers = map[string]interface{}{}
		}
		servers[orbitMCPName] = map[string]interface{}{
			"command": "npx",
			"args":    []string{"mcp-remote", mcpURL},
		}
		config["mcpServers"] = servers

	default:
		mcp, _ := config["mcp"].(map[string]interface{})
		if mcp == nil {
			mcp = map[string]interface{}{}
		}
		mcp[orbitMCPName] = map[string]interface{}{
			"type":    "local",
			"command": []string{"npx", "mcp-remote", mcpURL},
			"timeout": 120000,
			"enabled": true,
		}
		config["mcp"] = mcp
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return err
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, out, 0644) //nolint:gosec
}

func verifyOrbitAPI(client *http.Client, baseURL string, streams *iostreams.IOStreams) {
	cs := streams.Color()
	statusURL := baseURL + "orbit/status"

	resp, err := client.Get(statusURL) //nolint:noctx
	if err != nil {
		fmt.Fprintf(streams.StdOut, "%s Could not reach Orbit API: %v\n", cs.WarnIcon(), err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(streams.StdOut, "%s Orbit API reachable but response unreadable\n", cs.WarnIcon())
		return
	}

	status, _ := result["status"].(string)
	switch status {
	case "healthy":
		fmt.Fprintf(streams.StdOut, "%s Orbit API is healthy\n", cs.GreenCheck())
	case "":
		fmt.Fprintf(streams.StdOut, "%s Orbit API reachable but status unknown - knowledge_graph feature flag may be off\n", cs.WarnIcon())
		fmt.Fprintf(streams.StdOut, "    Enable it: /chatops gitlab run feature set --user=<your-username> knowledge_graph true\n")
	default:
		fmt.Fprintf(streams.StdOut, "%s Orbit API unhealthy (status: %s) - check glab auth\n", cs.WarnIcon(), status)
	}
}

func instanceURLFromBase(baseURL string) string {
	u := strings.TrimSuffix(baseURL, "/")
	u = strings.TrimSuffix(u, "/api/v4")
	return u
}

func printManualConfig(w io.Writer, instanceURL string) {
	fmt.Fprintf(w, "{\n  \"mcpServers\": {\n    \"gitlab-orbit\": {\n      \"command\": \"npx\",\n      \"args\": [\"mcp-remote\", \"%s/api/v4/orbit/mcp\"]\n    }\n  }\n}\n", instanceURL)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
