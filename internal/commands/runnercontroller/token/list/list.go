package list

import (
	"context"
	"fmt"
	"strconv"

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

	controllerID int64
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
		Use:   "list <controller-id> [flags]",
		Short: `List tokens for a runner controller. (EXPERIMENTAL)`,
		Args:  cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# List all tokens for runner controller 42
			$ glab runner-controller token list 42

			# List tokens as JSON
			$ glab runner-controller token list 42 --output json
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.Int64VarP(&opts.page, "page", "p", 1, "Page number.")
	fl.Int64VarP(&opts.perPage, "per-page", "P", 30, "Number of items per page.")
	fl.VarP(cmdutils.NewEnumValue([]string{"text", "json"}, "text", &opts.outputFormat), "output", "F", "Format output as: text, json.")

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

	listOpts := &gitlab.ListRunnerControllerTokensOptions{
		ListOptions: gitlab.ListOptions{
			Page:    o.page,
			PerPage: o.perPage,
		},
	}

	tokens, _, err := client.RunnerControllerTokens.ListRunnerControllerTokens(o.controllerID, listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return cmdutils.WrapError(err, "failed to list runner controller tokens")
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(tokens)
	default:
		return o.printTable(tokens)
	}
}

func (o *options) printTable(tokens []*gitlab.RunnerControllerToken) error {
	c := o.io.Color()
	table := tableprinter.NewTablePrinter()
	table.AddRow(c.Bold("ID"), c.Bold("Description"), c.Bold("Created At"), c.Bold("Updated At"))
	for _, t := range tokens {
		table.AddRow(t.ID, formatDescription(t.Description), t.CreatedAt, t.UpdatedAt)
	}
	fmt.Fprint(o.io.StdOut, table.Render())
	return nil
}

func formatDescription(desc string) string {
	if desc == "" {
		return "-"
	}
	return desc
}
