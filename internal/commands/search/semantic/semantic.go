package semantic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepoFunc func() (glrepo.Interface, error)

	query         string
	directoryPath string
	knn           int
	limit         int
	outputFormat  string
}

type semanticSearchResponse struct {
	Confidence string         `json:"confidence"`
	Results    []searchResult `json:"results"`
}

type searchResult struct {
	FilePath      string      `json:"file_path"`
	Path          string      `json:"path"`
	FileURL       string      `json:"file_url"`
	Score         float64     `json:"score"`
	Chunks        []codeChunk `json:"chunks"`
	SnippetRanges []codeChunk `json:"snippet_ranges"`
}

func (r searchResult) filePath() string {
	if r.FilePath != "" {
		return r.FilePath
	}
	return r.Path
}

func (r searchResult) chunks() []codeChunk {
	if len(r.Chunks) > 0 {
		return r.Chunks
	}
	return r.SnippetRanges
}

type codeChunk struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepoFunc: f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "semantic [flags]",
		Short: `Search project code using natural language.`,
		Long: heredoc.Doc(`
			Search project code using natural language (semantic similarity).

			Requires the project to have semantic code search enabled via GitLab Duo.
		`),
		Example: heredoc.Doc(`
			# Search for authentication-related code in the current project
			glab search semantic -q "authentication middleware"

			# Search within a specific directory
			glab search semantic -q "rate limiting" -d app/services/

			# Search in a specific project with JSON output
			glab search semantic -q "CI pipeline triggers" -R gitlab-org/gitlab --output json

			# Limit results
			glab search semantic -q "database migrations" --limit 5
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&opts.query, "query", "q", "", "Natural language search query. (required)")
	cmd.Flags().StringVarP(&opts.directoryPath, "directory-path", "d", "", "Restrict search to files under this path (e.g. app/services/).")
	cmd.Flags().IntVar(&opts.knn, "knn", 0, "Nearest neighbours to retrieve (1–100). Defaults to 64 server-side.")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 0, "Maximum number of results (1–100). Defaults to 20 server-side.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	_ = cmd.MarkFlagRequired("query")

	return cmd
}

func (o *options) run(ctx context.Context) error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	baseRepo, err := o.baseRepoFunc()
	if err != nil {
		return err
	}
	projectID := baseRepo.FullName()

	path := fmt.Sprintf("projects/%s/search/semantic", url.PathEscape(projectID))
	req, err := client.NewRequest(http.MethodGet, path, nil, []gitlab.RequestOptionFunc{
		gitlab.WithContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", o.query)
	if o.directoryPath != "" {
		q.Set("directory_path", o.directoryPath)
	}
	if o.knn > 0 {
		q.Set("knn", strconv.Itoa(o.knn))
	}
	if o.limit > 0 {
		q.Set("limit", strconv.Itoa(o.limit))
	}
	req.URL.RawQuery = q.Encode()

	var result semanticSearchResponse
	_, err = client.Do(req, &result)
	if err != nil {
		return fmt.Errorf("semantic search request failed: %w", err)
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(result)
	}

	return o.printText(projectID, &result)
}

func (o *options) printText(projectID string, result *semanticSearchResponse) error {
	c := o.io.Color()
	fmt.Fprintf(o.io.StdOut, "Searching for %q in %s...\n", o.query, projectID)
	fmt.Fprintf(o.io.StdOut, "Confidence: %s\n", result.Confidence)

	if len(result.Results) == 0 {
		fmt.Fprintln(o.io.StdOut, "\nNo results found.")
		return nil
	}

	fmt.Fprintln(o.io.StdOut)
	for _, r := range result.Results {
		fmt.Fprintf(o.io.StdOut, "%s  (score: %.2f)\n",
			c.Bold(r.filePath()), r.Score)
		for _, chunk := range r.chunks() {
			fmt.Fprintf(o.io.StdOut, "  Lines %d–%d:\n", chunk.StartLine, chunk.EndLine)
			for _, line := range splitLines(chunk.Content) {
				fmt.Fprintf(o.io.StdOut, "    %s\n", line)
			}
		}
		fmt.Fprintln(o.io.StdOut)
	}

	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
