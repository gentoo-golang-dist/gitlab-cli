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
	return nil
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
