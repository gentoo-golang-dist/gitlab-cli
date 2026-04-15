package issues

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/search/searchutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)
	apiClient    func(repoHost string) (*api.Client, error)

	query        string
	group        string
	outputFormat string

	page    int
	perPage int

	state        string
	confidential bool
	searchType   string
}

// NewCmdSearchIssues creates the `glab search issues` subcommand.
func NewCmdSearchIssues(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		baseRepo:  f.BaseRepo,
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:     "issues <query> [flags]",
		Short:   "Search for issues on GitLab.",
		Aliases: []string{"issue"},
		Long: heredoc.Doc(`Search for issues on GitLab at the instance, group, or project level.

Scope is resolved in the following order:
  1. Explicit project (-R / --repo) → project-level search.
  2. Explicit group (-g / --group)  → group-level search.
  3. Current git repo with a GitLab remote → project-level search.
  4. Otherwise                      → instance-level search.
`),
		Example: heredoc.Doc(`
			# Instance-level search
			glab search issues "authentication bug"

			# Group-level search
			glab search issues "auth" -g my-group

			# Project-level search (explicit)
			glab search issues "auth" -R gitlab-org/cli

			# Project-level search (inferred from current repo)
			glab search issues "auth"

			# Filter by state and output as JSON
			glab search issues "auth" --state closed --output json

			# Filter confidential issues
			glab search issues "secret" --confidential

			# Use advanced search
			glab search issues "label:~bug" --search-type advanced
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.query = args[0]

			group, err := cmdutils.GroupOverride(cmd)
			if err != nil {
				return err
			}
			opts.group = group

			return opts.run()
		},
	}

	// -R / --repo is handled by EnableRepoOverride (PersistentPreRunE + factory)
	cmdutils.EnableRepoOverride(cmd, f)

	// -g / --group
	cmd.Flags().StringP("group", "g", "", "Search within a specific group or subgroup.")

	// -F / --output
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	// Pagination
	cmd.Flags().IntVarP(&opts.page, "page", "p", 1, "Page number.")
	cmd.Flags().IntVarP(&opts.perPage, "per-page", "P", 20, "Number of results per page.")

	// Filters
	cmd.Flags().StringVar(&opts.state, "state", "", "Filter by state: opened, closed, all. Defaults to all states.")
	cmd.Flags().BoolVar(&opts.confidential, "confidential", false, "Filter confidential issues.")
	cmd.Flags().StringVar(&opts.searchType, "search-type", "", "Search type: basic, advanced, zoekt. Defaults to basic.")

	return cmd
}

func (o *options) run() error {
	// Determine search scope.
	scope, group, repo, err := searchutils.DetectScope(o.group, o.baseRepo)
	if err != nil {
		return err
	}

	// Resolve the API host from the repo, or default host when no repo is set.
	repoHost := ""
	if repo != nil {
		repoHost = repo.RepoHost()
	}

	c, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	gitlabClient := c.Lab()

	searchOpts := &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{
			Page:    int64(o.page),
			PerPage: int64(o.perPage),
		},
	}

	// Build extra RequestOptionFuncs for params that SearchOptions doesn't carry.
	var extraOpts []gitlab.RequestOptionFunc
	if o.state != "" {
		extraOpts = append(extraOpts, withQueryParam("state", o.state))
	}
	if o.confidential {
		extraOpts = append(extraOpts, withQueryParam("confidential", "true"))
	}
	if o.searchType != "" {
		extraOpts = append(extraOpts, withQueryParam("search_type", o.searchType))
	}

	var issues []*gitlab.Issue

	switch scope {
	case searchutils.ScopeProject:
		projectID := repo.FullName()
		issues, _, err = gitlabClient.Search.IssuesByProject(projectID, o.query, searchOpts, extraOpts...)
	case searchutils.ScopeGroup:
		issues, _, err = gitlabClient.Search.IssuesByGroup(group, o.query, searchOpts, extraOpts...)
	default: // ScopeInstance
		issues, _, err = gitlabClient.Search.Issues(o.query, searchOpts, extraOpts...)
	}
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		data, err := json.Marshal(issues)
		if err != nil {
			return err
		}
		fmt.Fprintln(o.io.StdOut, string(data))
		return nil
	}

	// Text output
	scopeLabel := scopeDescription(scope, group, repo)
	if len(issues) == 0 {
		fmt.Fprintf(o.io.StdOut, "No issues found for %q in %s\n", o.query, scopeLabel)
		return nil
	}

	fmt.Fprintf(o.io.StdOut, "Showing %d issue(s) matching %q in %s\n\n", len(issues), o.query, scopeLabel)
	fmt.Fprint(o.io.StdOut, searchutils.DisplayIssues(o.io, issues))
	fmt.Fprintln(o.io.StdOut)
	return nil
}

// withQueryParam returns a RequestOptionFunc that appends a query parameter.
// Note: q.Encode() already percent-encodes values, so we pass value directly
// without pre-escaping.
func withQueryParam(key, value string) gitlab.RequestOptionFunc {
	return func(req *retryablehttp.Request) error {
		q := req.URL.Query()
		q.Set(key, value)
		req.URL.RawQuery = q.Encode()
		return nil
	}
}

func scopeDescription(scope searchutils.SearchScope, group string, repo glrepo.Interface) string {
	switch scope {
	case searchutils.ScopeProject:
		if repo != nil {
			return fmt.Sprintf("project %s", repo.FullName())
		}
	case searchutils.ScopeGroup:
		return fmt.Sprintf("group %s", group)
	}
	return "the GitLab instance"
}
