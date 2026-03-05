package list

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

type options struct {
	io        *iostreams.IOStreams
	apiClient func(repoHost string) (*api.Client, error)

	page         int64
	perPage      int64
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: `List runner controllers. (EXPERIMENTAL)`,
		Args:  cobra.NoArgs,
		Example: heredoc.Doc(`
			# List all runner controllers
			$ glab runner-controller list

			# List runner controllers as JSON
			$ glab runner-controller list --output json
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.Int64VarP(&opts.page, "page", "p", 1, "Page number.")
	fl.Int64VarP(&opts.perPage, "per-page", "P", 30, "Number of items per page.")
	fl.VarP(cmdutils.NewEnumValue([]string{"text", "json"}, "text", &opts.outputFormat), "output", "F", "Format output as: text, json.")

	return cmd
}

func (o *options) run(ctx context.Context) error {
	apiClient, err := o.apiClient("")
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	listOpts := &gitlab.ListRunnerControllersOptions{
		ListOptions: gitlab.ListOptions{
			Page:    o.page,
			PerPage: o.perPage,
		},
	}

	controllers, _, err := client.RunnerControllers.ListRunnerControllers(listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return err
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(controllers)
	default:
		return o.printTable(controllers)
	}
}

func (o *options) printTable(controllers []*gitlab.RunnerController) error {
	c := o.io.Color()
	table := tableprinter.NewTablePrinter()
	table.AddRow(c.Bold("ID"), c.Bold("Description"), c.Bold("State"), c.Bold("Created At"), c.Bold("Updated At"))
	for _, rc := range controllers {
		table.AddRow(rc.ID, rc.Description, formatState(c, rc.State), rc.CreatedAt, rc.UpdatedAt)
	}
	fmt.Fprint(o.io.StdOut, table.Render())
	return nil
}

func formatState(c *iostreams.ColorPalette, state gitlab.RunnerControllerStateValue) string {
	switch state {
	case gitlab.RunnerControllerStateDisabled:
		return c.Gray(string(state))
	case gitlab.RunnerControllerStateDryRun:
		return c.Yellow(string(state))
	case gitlab.RunnerControllerStateEnabled:
		return c.Green(string(state))
	default:
		return string(state)
	}
}
