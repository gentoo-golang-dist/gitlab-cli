package dag

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

var (
	//go:embed long.md
	longHelp string
	//go:embed example.md
	exampleHelp string
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)

	path           string
	compiled       bool
	showStageEdges bool
	listenAddr     string
	output         string
	source         string
	branch         string
	tagName        string
	sourceBranch   string
	targetBranch   string
	vars           []string
}

func NewCmdDag(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,

		listenAddr:     "localhost:0",
		showStageEdges: true,
		output:         "browser",
	}

	cmd := &cobra.Command{
		Use:     "dag [path] [flags]",
		Short:   "Generate a DAG visualization of a CI/CD pipeline.",
		Long:    longHelp,
		Example: strings.Trim(exampleHelp, "\n\r"),
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.BoolVar(&opts.compiled, "compiled", false,
		"Use the GitLab API to get the fully expanded configuration (resolves include: directives).")
	fl.BoolVar(&opts.showStageEdges, "stage-edges", true,
		"Show implicit stage ordering edges between consecutive stages.")
	fl.StringVar(&opts.listenAddr, "listen-addr", opts.listenAddr,
		"Address for the local HTTP server.")
	fl.StringVarP(&opts.output, "output", "o", opts.output,
		"Output mode: 'browser' to open in browser, 'svg' to print SVG to stdout.")
	fl.StringVar(&opts.source, "source", "",
		"Set CI_PIPELINE_SOURCE to simulate a pipeline type (e.g. push, merge_request_event, schedule, web, api).")
	fl.StringVar(&opts.branch, "branch", "",
		"Set CI_COMMIT_BRANCH. Implies --source push if no --source is set.")
	fl.StringVar(&opts.tagName, "tag", "",
		"Set CI_COMMIT_TAG. Implies --source push if no --source is set.")
	fl.StringVar(&opts.sourceBranch, "source-branch", "",
		"Set CI_MERGE_REQUEST_SOURCE_BRANCH_NAME. Implies --source merge_request_event.")
	fl.StringVar(&opts.targetBranch, "target-branch", "",
		"Set CI_MERGE_REQUEST_TARGET_BRANCH_NAME. Implies --source merge_request_event.")
	fl.StringArrayVar(&opts.vars, "var", nil,
		"Set a CI variable as KEY=VALUE. Can be specified multiple times.")

	return cmd
}

func (o *options) complete(args []string) {
	if len(args) == 1 {
		o.path = args[0]
	} else {
		o.path = ".gitlab-ci.yml"
	}
}

func (o *options) validate() error {
	if o.output != "browser" && o.output != "svg" {
		return cmdutils.FlagError{Err: fmt.Errorf("invalid output mode %q: must be 'browser' or 'svg'", o.output)}
	}

	for _, v := range o.vars {
		if !strings.Contains(v, "=") {
			return cmdutils.FlagError{Err: fmt.Errorf("invalid --var format %q: expected KEY=VALUE", v)}
		}
	}

	return nil
}

func (o *options) hasSimulationFlags() bool {
	return o.source != "" || o.branch != "" || o.tagName != "" ||
		o.sourceBranch != "" || o.targetBranch != "" || len(o.vars) > 0
}

func (o *options) run(ctx context.Context) error {
	yamlContent, err := o.loadYAML()
	if err != nil {
		return err
	}

	pipeline, err := ParsePipeline(yamlContent)
	if err != nil {
		return err
	}
	if len(pipeline.Jobs) == 0 {
		return fmt.Errorf("no jobs found in %s", o.path)
	}

	if o.hasSimulationFlags() {
		if err := o.applyRulesFilter(pipeline); err != nil {
			return err
		}
	}

	d2Source := BuildD2Source(pipeline, o.showStageEdges)

	svgData, err := renderSVG(ctx, d2Source)
	if err != nil {
		return fmt.Errorf("rendering diagram: %w", err)
	}

	if o.output == "svg" {
		_, err := o.io.StdOut.Write(svgData)
		return err
	}

	srv := &dagServer{
		io:         o.io,
		svgData:    svgData,
		listenAddr: o.listenAddr,
	}
	return srv.Run(ctx)
}

func (o *options) loadYAML() ([]byte, error) {
	if o.compiled {
		return o.loadCompiledYAML()
	}

	content, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: no such file or directory", o.path)
		}
		return nil, err
	}
	return content, nil
}

func (o *options) loadCompiledYAML() ([]byte, error) {
	client, err := o.gitlabClient()
	if err != nil {
		return nil, err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return nil, fmt.Errorf("you must be in a GitLab project repository to use --compiled: %w", err)
	}

	project, err := repo.Project(client)
	if err != nil {
		return nil, fmt.Errorf("you must be in a GitLab project repository to use --compiled: %w", err)
	}

	content, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: no such file or directory", o.path)
		}
		return nil, err
	}

	o.io.LogInfo("Compiling CI/CD configuration via GitLab API...")

	dryRun := true
	result, _, err := client.Validate.ProjectNamespaceLint(
		project.ID,
		&gitlab.ProjectNamespaceLintOptions{
			Content:     new(string(content)),
			DryRun:      &dryRun,
			IncludeJobs: new(bool),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("GitLab API error: %w", err)
	}

	if !result.Valid {
		return nil, fmt.Errorf("CI/CD configuration is invalid: %s", strings.Join(result.Errors, "; "))
	}

	return []byte(result.MergedYaml), nil
}

func (o *options) applyRulesFilter(pipeline *Pipeline) error {
	cfg := o.buildSimulationConfig()
	vars := BuildVariables(cfg)

	if err := EvalWorkflowRules(pipeline.WorkflowRules, vars); err != nil {
		return err
	}

	pipeline.Jobs = FilterJobs(pipeline.Jobs, vars)
	if len(pipeline.Jobs) == 0 {
		return fmt.Errorf("no jobs would run for this pipeline configuration")
	}

	// Re-filter stages to only those with remaining jobs.
	usedStages := make(map[string]bool)
	for _, j := range pipeline.Jobs {
		usedStages[j.Stage] = true
	}
	var filteredStages []string
	for _, s := range pipeline.Stages {
		if usedStages[s] {
			filteredStages = append(filteredStages, s)
		}
	}
	pipeline.Stages = filteredStages

	// Clean up needs references to filtered-out jobs.
	jobExists := make(map[string]bool)
	for _, j := range pipeline.Jobs {
		jobExists[j.Name] = true
	}
	for i, j := range pipeline.Jobs {
		var validNeeds []string
		for _, n := range j.Needs {
			if jobExists[n] {
				validNeeds = append(validNeeds, n)
			}
		}
		pipeline.Jobs[i].Needs = validNeeds
	}

	return nil
}

func (o *options) buildSimulationConfig() SimulationConfig {
	cfg := SimulationConfig{
		Source:       o.source,
		Branch:       o.branch,
		Tag:          o.tagName,
		SourceBranch: o.sourceBranch,
		TargetBranch: o.targetBranch,
		ExtraVars:    make(map[string]string),
	}
	for _, v := range o.vars {
		parts := strings.SplitN(v, "=", 2)
		cfg.ExtraVars[parts[0]] = parts[1]
	}
	return cfg
}

func renderSVG(ctx context.Context, d2Source string) ([]byte, error) {
	// D2 requires a slog.Logger in the context.
	ctx = log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("initializing text measurer: %w", err)
	}

	compileOpts := &d2lib.CompileOptions{
		Ruler: ruler,
		LayoutResolver: func(_ string) (d2graph.LayoutGraph, error) {
			return d2dagrelayout.DefaultLayout, nil
		},
	}

	diagram, _, err := d2lib.Compile(ctx, d2Source, compileOpts, nil)
	if err != nil {
		return nil, fmt.Errorf("compiling diagram: %w", err)
	}

	svgData, err := d2svg.Render(diagram, &d2svg.RenderOpts{})
	if err != nil {
		return nil, fmt.Errorf("rendering SVG: %w", err)
	}

	return svgData, nil
}
