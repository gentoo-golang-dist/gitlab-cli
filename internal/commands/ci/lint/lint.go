package lint

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

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

	path              string
	ref               string
	dryRun            bool
	includeJobs       bool
	includeMergedYAML bool
}

func NewCmdLint(f cmdutils.Factory) *cobra.Command {
	opts := newOptions(f)
	pipelineCILintCmd := &cobra.Command{
		Use:   "lint",
		Short: "Checks if your `.gitlab-ci.yml` file is valid.",
		Args:  cobra.MaximumNArgs(1),
		Example: heredoc.Doc(`
			# Uses .gitlab-ci.yml in the current directory
			glab ci lint
			glab ci lint .gitlab-ci.yml
			glab ci lint path/to/.gitlab-ci.yml

			# Output the fully expanded CI/CD configuration
			glab ci lint --include-merged-yaml --dry-run --ref scratch`),
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
	pipelineCILintCmd.Flags().BoolVar(&opts.includeMergedYAML, "include-merged-yaml", false, "Output the fully expanded CI/CD configuration instead of the validation summary.")
	pipelineCILintCmd.Flags().StringVar(&opts.ref, "ref", "", "When 'dry-run' is true, sets the branch or tag context for validating the CI/CD YAML configuration.")

	return pipelineCILintCmd
}

func newOptions(f cmdutils.Factory) options {
	return options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
}

// RunMergedYAML reuses ci lint to print the merged YAML response for a local CI file.
func RunMergedYAML(f cmdutils.Factory, path string) error {
	opts := newOptions(f)
	opts.path = path
	opts.includeMergedYAML = true

	return opts.run()
}

func (o *options) complete(args []string) {
	if len(args) == 1 {
		o.path = args[0]
	} else {
		o.path = ".gitlab-ci.yml"
	}
}

func (o *options) run() error {
	out := o.io.StdOut
	if !o.includeMergedYAML {
		fmt.Fprintln(out, "Validating...")
	}

	lintResult, err := o.lint()
	if err != nil {
		return err
	}

	if !lintResult.Valid {
		fmt.Fprintln(out, o.io.Color().Red(o.path+" is invalid."))
		for i, err := range lintResult.Errors {
			i++
			fmt.Fprintln(out, i, err)
		}
		return cmdutils.SilentError
	}

	if o.includeMergedYAML {
		_, err = fmt.Fprint(out, lintResult.MergedYaml)
		return err
	}

	fmt.Fprintln(out, o.io.Color().GreenCheck(), "CI/CD YAML is valid!")
	return nil
}

func (o *options) lint() (*gitlab.ProjectLintResult, error) {
	client, err := o.gitlabClient()
	if err != nil {
		return nil, err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return nil, fmt.Errorf("You must be in a GitLab project repository for this action.\nError: %s", err)
	}

	project, err := repo.Project(client)
	if err != nil {
		return nil, fmt.Errorf("You must be in a GitLab project repository for this action.\nError: %s", err)
	}

	content, err := o.loadContent()
	if err != nil {
		return nil, err
	}

	lintOpts := &gitlab.ProjectNamespaceLintOptions{
		Content:     new(string(content)),
		DryRun:      new(o.dryRun),
		IncludeJobs: new(o.includeJobs),
	}
	if o.ref != "" {
		lintOpts.Ref = new(o.ref)
	}

	lintResult, _, err := client.Validate.ProjectNamespaceLint(project.ID, lintOpts)
	if err != nil {
		return nil, err
	}

	return lintResult, nil
}

func (o *options) loadContent() ([]byte, error) {
	if git.IsValidURL(o.path) {
		resp, err := http.Get(o.path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var stdout bytes.Buffer
		_, err = io.Copy(&stdout, resp.Body)
		if err != nil {
			return nil, err
		}

		return stdout.Bytes(), nil
	}

	content, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: no such file or directory.", o.path)
		}
		return nil, err
	}

	return content, nil
}
