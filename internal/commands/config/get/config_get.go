package get

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io     *iostreams.IOStreams
	config func() config.Config

	hostname string
	key      string
}

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		config: f.Config,
		io:     f.IO(),
	}

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Prints the value of a given configuration key.",
		Long:  ``,
		Example: heredoc.Doc(`
  		$ glab config get editor
  		> vim

  		$ glab config get glamour_style
  		> notty
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			return opts.run()
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&opts.hostname, "host", "", "", "Get per-host setting.")
	fl.BoolP("global", "g", false, "Read from global config file (~/.config/glab-cli/config.yml). (default checks 'Environment variables → Local → Global')")

	return cmd
}

func (o *options) complete(args []string) {
	o.key = args[0]
}

func (o *options) run() error {
	cfg := o.config()

	val, err := cfg.Get(o.hostname, o.key)
	if err != nil {
		return err
	}

	if val != "" {
		fmt.Fprintf(o.io.StdOut, "%s\n", val)
	}
	return nil
}
