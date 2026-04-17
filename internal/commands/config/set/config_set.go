package set

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	config func() config.Config

	hostname string
	isGlobal bool
	key      string
	value    string
}

func NewCmdSet(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		config: f.Config,
	}

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Updates configuration with the value of a given key.",
		Long: `Update the configuration by setting a key to a value.
Use 'glab config set --global' to set a global config.
Specifying the '--host' flag also saves in the global configuration file.
`,
		Example: heredoc.Doc(`
glab config set editor vim
glab config set token xxxxx --host gitlab.com
glab config set check_update false --global`),
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.complete(args)
			return opts.run()
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&opts.hostname, "host", "", "", "Set per-host setting.")
	fl.BoolVarP(&opts.isGlobal, "global", "g", false, "Write to global '~/.config/glab-cli/config.yml' file rather than the repository's '.git/glab-cli/config.yml' file.")
	return cmd
}

func (o *options) complete(args []string) {
	o.key = args[0]
	o.value = args[1]
}

func (o *options) run() error {
	cfg := o.config()

	localCfg, _ := cfg.Local()

	var err error
	if o.isGlobal || o.hostname != "" {
		err = cfg.Set(o.hostname, o.key, o.value)
	} else {
		err = localCfg.Set(o.key, o.value)
	}

	if err != nil {
		return fmt.Errorf("failed to set %q to %q: %w", o.key, o.value, err)
	}

	if o.isGlobal || o.hostname != "" {
		err = cfg.Write()
	} else {
		err = localCfg.Write()
	}

	if err != nil {
		return fmt.Errorf("failed to write configuration to disk: %w", err)
	}
	return nil
}
