package lintcompile

import (
	"fmt"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)


type Result struct {
	MergedYaml   string
	RunnableJobs []string
}

// Options controls a CompileAndList call.
type Options struct {
	// Without a Ref, DryRun uses the project's default branch.
	Ref string

	DryRun bool

	IncludeJobs bool
}

// lintResponse matches the raw JSON shape of POST /projects/:id/ci/lint
type lintResponse struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
	MergedYaml string   `json:"merged_yaml"`
	Jobs       []struct {
		Name  string `json:"name"`
		Stage string `json:"stage"`
	} `json:"jobs"`
}

type lintRequest struct {
	Content     string `json:"content"`
	DryRun      bool   `json:"dry_run,omitempty"`
	IncludeJobs bool   `json:"include_jobs,omitempty"`
	Ref         string `json:"ref,omitempty"`
}

// CompileAndList calls the GitLab project-namespace lint API to fully resolve
// a .gitlab-ci.yml (includes, extends, defaults)
func CompileAndList(client *gitlab.Client, projectID int64, content []byte, opts Options) (*Result, error) {
	body := lintRequest{
		Content:     string(content),
		DryRun:      opts.DryRun,
		IncludeJobs: opts.IncludeJobs,
		Ref:         opts.Ref,
	}
	path := fmt.Sprintf("projects/%d/ci/lint", projectID)
	req, err := client.NewRequest(http.MethodPost, path, body, nil)
	if err != nil {
		return nil, fmt.Errorf("building lint request: %w", err)
	}

	var resp lintResponse
	if _, err := client.Do(req, &resp); err != nil {
		return nil, fmt.Errorf("GitLab lint API error: %w", err)
	}

	if !resp.Valid {
		return nil, fmt.Errorf("CI/CD configuration is invalid: %s", strings.Join(resp.Errors, "; "))
	}

	result := &Result{MergedYaml: resp.MergedYaml}
	if opts.IncludeJobs {
		names := make([]string, 0, len(resp.Jobs))
		for _, j := range resp.Jobs {
			names = append(names, j.Name)
		}
		result.RunnableJobs = names
	}
	return result, nil
}

// CompileYAML is the simple path used by `glab ci config compile`
// lint only, returning merged YAML.
func CompileYAML(client *gitlab.Client, projectID int64, content []byte) (string, error) {
	s := string(content)
	dryRun := false
	includeJobs := false

	result, _, err := client.Validate.ProjectNamespaceLint(
		projectID,
		&gitlab.ProjectNamespaceLintOptions{
			Content:     &s,
			DryRun:      &dryRun,
			IncludeJobs: &includeJobs,
		},
	)
	if err != nil {
		return "", fmt.Errorf("GitLab lint API error: %w", err)
	}

	if !result.Valid {
		return "", fmt.Errorf("CI/CD configuration is invalid: %s", strings.Join(result.Errors, "; "))
	}
	return result.MergedYaml, nil
}
