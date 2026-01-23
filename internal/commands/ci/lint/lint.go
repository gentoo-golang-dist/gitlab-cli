package lint

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)

	path        string
	ref         string
	dryRun      bool
	includeJobs bool

	// Component context values for interpolation
	componentName      string
	componentVersion   string
	componentSHA       string
	componentReference string
}

func NewCmdLint(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	pipelineCILintCmd := &cobra.Command{
		Use:   "lint",
		Short: "Checks if your `.gitlab-ci.yml` file is valid.",
		Args:  cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# Uses .gitlab-ci.yml in the current directory
			$ glab ci lint
			$ glab ci lint .gitlab-ci.yml
			$ glab ci lint path/to/.gitlab-ci.yml

			# Lint a CI/CD component template with component context values
			$ glab ci lint templates/my-component.yml --component-name=my-component --component-version=1.0.0
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			return opts.run()
		},
	}

	pipelineCILintCmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "", false, "Run pipeline creation simulation.")
	pipelineCILintCmd.Flags().BoolVarP(&opts.includeJobs, "include-jobs", "", false, "Response includes the list of jobs that would exist in a static check or pipeline simulation.")
	pipelineCILintCmd.Flags().StringVar(&opts.ref, "ref", "", "When 'dry-run' is true, sets the branch or tag context for validating the CI/CD YAML configuration.")

	// Component context flags for linting CI/CD component templates
	pipelineCILintCmd.Flags().StringVar(&opts.componentName, "component-name", "", "Component name to use for $[[ component.name ]] interpolation.")
	pipelineCILintCmd.Flags().StringVar(&opts.componentVersion, "component-version", "", "Component version to use for $[[ component.version ]] interpolation.")
	pipelineCILintCmd.Flags().StringVar(&opts.componentSHA, "component-sha", "", "Component SHA to use for $[[ component.sha ]] interpolation.")
	pipelineCILintCmd.Flags().StringVar(&opts.componentReference, "component-reference", "", "Component reference to use for $[[ component.reference ]] interpolation.")

	return pipelineCILintCmd
}

func (o *options) complete(args []string) {
	if len(args) == 1 {
		o.path = args[0]
	} else {
		o.path = ".gitlab-ci.yml"
	}
}

// replaceComponentContext replaces $[[ component.* ]] expressions with the provided values.
// This allows linting CI/CD component templates that use component context metadata.
func (o *options) replaceComponentContext(content string) string {
	// Build a map of component context values
	replacements := map[string]string{
		"name":      o.componentName,
		"version":   o.componentVersion,
		"sha":       o.componentSHA,
		"reference": o.componentReference,
	}

	// Replace each component context expression if a value was provided
	for field, value := range replacements {
		if value != "" {
			// Match $[[ component.field ]] with optional whitespace
			pattern := regexp.MustCompile(`\$\[\[\s*component\.` + regexp.QuoteMeta(field) + `\s*\]\]`)
			content = pattern.ReplaceAllString(content, value)
		}
	}

	return content
}

// hasComponentContext checks if any component context flags were provided
func (o *options) hasComponentContext() bool {
	return o.componentName != "" || o.componentVersion != "" ||
		o.componentSHA != "" || o.componentReference != ""
}

func (o *options) run() error {
	var err error
	out := o.io.StdOut
	c := o.io.Color()

	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return fmt.Errorf("You must be in a GitLab project repository for this action.\nError: %s", err)
	}

	project, err := repo.Project(client)
	if err != nil {
		return fmt.Errorf("You must be in a GitLab project repository for this action.\nError: %s", err)
	}

	projectID := project.ID

	var content []byte
	var stdout bytes.Buffer

	if git.IsValidURL(o.path) {
		resp, err := http.Get(o.path)
		if err != nil {
			return err
		}
		_, err = io.Copy(&stdout, resp.Body)
		if err != nil {
			return err
		}
		content = stdout.Bytes()
	} else {
		content, err = os.ReadFile(o.path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%s: no such file or directory.", o.path)
			}
			return err
		}
	}

	fmt.Fprintln(o.io.StdOut, "Validating...")

	// Replace component context expressions if any component context flags were provided
	contentStr := string(content)
	if o.hasComponentContext() {
		contentStr = o.replaceComponentContext(contentStr)
	}

	lintOpts := &gitlab.ProjectNamespaceLintOptions{
		Content:     gitlab.Ptr(contentStr),
		DryRun:      gitlab.Ptr(o.dryRun),
		IncludeJobs: gitlab.Ptr(o.includeJobs),
	}
	// Only include Ref if it was explicitly set by the user
	if o.ref != "" {
		lintOpts.Ref = gitlab.Ptr(o.ref)
	}

	lint, _, err := client.Validate.ProjectNamespaceLint(
		projectID,
		lintOpts,
	)
	if err != nil {
		return err
	}

	if !lint.Valid {
		fmt.Fprintln(out, c.Red(o.path+" is invalid."))
		for i, err := range lint.Errors {
			i++
			fmt.Fprintln(out, i, err)
		}
		return cmdutils.SilentError
	}
	fmt.Fprintln(out, c.GreenCheck(), "CI/CD YAML is valid!")
	return nil
}
