package visualize

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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

	path       string
	web        bool
	listenAddr string
	output     string
}

func NewCmdVisualize(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,

		listenAddr: "localhost:0",
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
	fl.StringVar(&opts.listenAddr, "listen-addr", opts.listenAddr,
		"Address for the local HTTP server. Only used with --web.")
	fl.StringVarP(&opts.output, "output", "o", outputDefault,
		"Output mode: 'svg' prints raw SVG to stdout. Defaults to opening the rendered SVG in a browser.")

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
	return nil
}

func (o *options) run(ctx context.Context) error {
	mergedYaml, err := o.loadCompiledYAML()
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

	d2Source := BuildD2Source(pipeline, true)

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

// loadCompiledYAML reads the local file and calls the GitLab lint API
// so includes, extends, and defaults are fully resolved before parsing.
func (o *options) loadCompiledYAML() ([]byte, error) {
	content, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: no such file or directory", o.path)
		}
		return nil, err
	}

	client, err := o.gitlabClient()
	if err != nil {
		return nil, err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return nil, fmt.Errorf("ci visualize needs a GitLab project to compile the configuration; run from inside a cloned repo or set --repo: %w", err)
	}

	project, err := repo.Project(client)
	if err != nil {
		return nil, fmt.Errorf("ci visualize needs a GitLab project to compile the configuration; run from inside a cloned repo or set --repo: %w", err)
	}

	fmt.Fprintln(o.io.StdErr, "Compiling CI/CD configuration via GitLab API...")

	return compileViaLintAPI(client, project.ID, content)
}

type lintRequest struct {
	Content string `json:"content"`
}

type lintResponse struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors"`
	MergedYaml string   `json:"merged_yaml"`
}

func compileViaLintAPI(client *gitlab.Client, projectID int64, content []byte) ([]byte, error) {
	path := fmt.Sprintf("projects/%d/ci/lint", projectID)
	req, err := client.NewRequest(http.MethodPost, path, lintRequest{Content: string(content)}, nil)
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

	return []byte(resp.MergedYaml), nil
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
