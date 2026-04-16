package visualize

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/shared/lintcompile"
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

const (
	outputDefault = ""
	outputSVG     = "svg"
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)

	path           string
	web            bool
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

func NewCmdVisualize(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,

		listenAddr:     "localhost:0",
		showStageEdges: true,
	}

	cmd := &cobra.Command{
		Use:     "visualize [path] [flags]",
		Short:   "Visualize a CI/CD pipeline as a DAG.",
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
	fl.BoolVar(&opts.web, "web", false,
		"Start an interactive local HTTP server with pan/zoom instead of opening an SVG file.")
	fl.BoolVar(&opts.showStageEdges, "stage-edges", true,
		"Show implicit stage ordering edges between consecutive stages.")
	fl.StringVar(&opts.listenAddr, "listen-addr", opts.listenAddr,
		"Address for the local HTTP server. Only used with --web.")
	fl.StringVarP(&opts.output, "output", "o", outputDefault,
		"Output mode: 'svg' prints raw SVG to stdout. Defaults to opening the rendered SVG in a browser.")
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
	if o.output != outputDefault && o.output != outputSVG {
		return cmdutils.FlagError{Err: fmt.Errorf("invalid output mode %q: must be 'svg'", o.output)}
	}
	if o.output == outputSVG && o.web {
		return cmdutils.FlagError{Err: fmt.Errorf("--output svg cannot be combined with --web")}
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
	mergedYaml, runnableJobs, err := o.loadCompiledYAML()
	if err != nil {
		return err
	}

	pipeline, err := ParsePipeline(mergedYaml)
	if err != nil {
		return err
	}
	if len(pipeline.Jobs) == 0 {
		return fmt.Errorf("no jobs found in %s", o.path)
	}

	if runnableJobs != nil {
		applyAPIFilter(pipeline, runnableJobs)
		if len(pipeline.Jobs) == 0 {
			return fmt.Errorf("no jobs would run for this pipeline configuration")
		}
	}

	d2Source := BuildD2Source(pipeline, o.showStageEdges)

	svgData, err := renderSVG(ctx, d2Source)
	if err != nil {
		return fmt.Errorf("rendering diagram: %w", err)
	}

	return o.emit(ctx, svgData)
}

func (o *options) emit(ctx context.Context, svgData []byte) error {
	switch {
	case o.output == outputSVG:
		_, err := o.io.StdOut.Write(svgData)
		return err
	case o.web:
		srv := &visualizeServer{
			io:         o.io,
			svgData:    svgData,
			listenAddr: o.listenAddr,
		}
		return srv.Run(ctx)
	default:
		return o.openInBrowser(svgData)
	}
}

func (o *options) openInBrowser(svgData []byte) error {
	f, err := os.CreateTemp("", "glab-visualize-*.svg")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := f.Write(svgData); err != nil {
		f.Close() //nolint:errcheck
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	abs, err := filepath.Abs(f.Name())
	if err != nil {
		abs = f.Name()
	}
	url := "file://" + filepath.ToSlash(abs)

	fmt.Fprintf(o.io.StdErr, "Rendered pipeline to %s\n", abs)
	if err := browser.OpenURL(url); err != nil {
		o.io.LogError("Failed to open browser:", err)
		fmt.Fprintf(o.io.StdErr, "Open %s manually in your browser.\n", url)
	}
	return nil
}

// loadCompiledYAML reads the local file, injects any simulation variables
// and calls the GitLab lint API
func (o *options) loadCompiledYAML() ([]byte, []string, error) {
	content, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%s: no such file or directory", o.path)
		}
		return nil, nil, err
	}

	client, err := o.gitlabClient()
	if err != nil {
		return nil, nil, err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return nil, nil, fmt.Errorf("ci visualize needs a GitLab project to compile the configuration; run from inside a cloned repo or set --repo: %w", err)
	}

	project, err := repo.Project(client)
	if err != nil {
		return nil, nil, fmt.Errorf("ci visualize needs a GitLab project to compile the configuration; run from inside a cloned repo or set --repo: %w", err)
	}

	simulating := o.hasSimulationFlags()
	if simulating {
		vars := BuildVariables(o.buildSimulationConfig())
		content, err = injectVariables(content, vars)
		if err != nil {
			return nil, nil, err
		}
	}

	fmt.Fprintln(o.io.StdErr, "Compiling CI/CD configuration via GitLab API...")

	result, err := lintcompile.CompileAndList(client, project.ID, content, lintcompile.Options{
		DryRun:      simulating,
		IncludeJobs: simulating,
	})
	if err != nil {
		return nil, nil, err
	}
	return []byte(result.MergedYaml), result.RunnableJobs, nil
}

// applyAPIFilter parses pipeline with the job-name list 
func applyAPIFilter(pipeline *Pipeline, runnableJobs []string) {
	allowed := make(map[string]bool, len(runnableJobs))
	for _, name := range runnableJobs {
		allowed[name] = true
	}

	kept := pipeline.Jobs[:0]
	for _, j := range pipeline.Jobs {
		if allowed[j.Name] {
			kept = append(kept, j)
		}
	}
	pipeline.Jobs = kept

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

	jobExists := make(map[string]bool, len(pipeline.Jobs))
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
