package delete

import (
	"context"
	"errors"
	"fmt"
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
	force        bool
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:   "delete <controller-id> [flags]",
		Short: `Delete a scope from a runner controller. (EXPERIMENTAL)`,
		Long: heredoc.Docf(`
			Delete a scope from a runner controller. This is an admin-only feature.

			Currently, only instance-level scopes are supported. Use the --instance flag
			to remove an instance-level scope from the runner controller.
			%s`, text.ExperimentalString),
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Remove an instance-level scope from runner controller 42 (with confirmation)
			$ glab runner-controller scope delete 42 --instance

			# Remove an instance-level scope without confirmation
			$ glab runner-controller scope delete 42 --instance --force
		`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}
			if err := opts.validate(cmd.Context()); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.BoolVar(&opts.instance, "instance", false, "Remove an instance-level scope.")
	fl.BoolVarP(&opts.force, "force", "f", false, "Skip confirmation prompt.")

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

func (o *options) validate(ctx context.Context) error {
	if !o.force {
		if !o.io.PromptEnabled() {
			return cmdutils.FlagError{Err: errors.New("--force required when not running interactively")}
		}

		var confirmed bool
		err := o.io.Confirm(ctx, &confirmed, fmt.Sprintf("Remove instance-level scope from runner controller %d?", o.controllerID))
		if err != nil {
			return err
		}

		if !confirmed {
			return cmdutils.CancelError()
		}
	}
	return nil
}

func (o *options) run(ctx context.Context) error {
	apiClient, err := o.apiClient("")
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	_, err = client.RunnerControllerScopes.RemoveRunnerControllerInstanceScope(o.controllerID, gitlab.WithContext(ctx))
	if err != nil {
		return cmdutils.WrapError(err, "failed to remove instance-level scope")
	}

	o.io.LogInfof("Removed instance-level scope from runner controller %d\n", o.controllerID)
	return nil
}
