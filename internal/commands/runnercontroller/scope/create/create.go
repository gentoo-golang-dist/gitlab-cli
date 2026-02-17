package create

import (
	"context"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type options struct {
	io        *iostreams.IOStreams
	apiClient func(repoHost string) (*api.Client, error)

	controllerID int64
	instance     bool
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:   "create <controller-id> [flags]",
		Short: `Create a scope for a runner controller. (EXPERIMENTAL)`,
		Long: heredoc.Docf(`
			Creates a scope for a runner controller. This is an administrator-only feature.

			Only instance-level scopes are supported. Use the %[1]s--instance%[1]s flag
			to add an instance-level scope. With an instance-level scope, the runner
			controller can evaluate jobs for all runners in the GitLab instance.
			%s`, "`", text.ExperimentalString),
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Add an instance-level scope to runner controller 42
			$ glab runner-controller scope create 42 --instance

			# Add an instance-level scope and output as JSON
			$ glab runner-controller scope create 42 --instance --output json
		`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.BoolVar(&opts.instance, "instance", false, "Add an instance-level scope.")
	fl.VarP(cmdutils.NewEnumValue([]string{"text", "json"}, "text", &opts.outputFormat), "output", "F", "Format output as: text, json.")

	cobra.CheckErr(cmd.MarkFlagRequired("instance"))

	return cmd
}

func (o *options) complete(args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return cmdutils.WrapError(err, "invalid runner controller ID")
	}
	o.controllerID = id
	return nil
}

func (o *options) run(ctx context.Context) error {
	apiClient, err := o.apiClient("")
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	scoping, _, err := client.RunnerControllerScopes.AddRunnerControllerInstanceScope(o.controllerID, gitlab.WithContext(ctx))
	if err != nil {
		return cmdutils.WrapError(err, "failed to add instance-level scope")
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(scoping)
	default:
		o.io.LogInfof("Added instance-level scope to runner controller %d\n", o.controllerID)
		return nil
	}
}
