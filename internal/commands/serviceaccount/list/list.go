package list

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	group        string
	outputFormat string
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List service accounts in a group.",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		Long: heredoc.Doc(`
			List all service accounts for a group.

			This API endpoint works on top-level groups only. It does not
			work on subgroups.
		`),
		Example: heredoc.Doc(`
		# List service accounts in a group
		glab service-account list --group my-group

		# List service accounts as JSON
		glab service-account list --group my-group --output json`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run()
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "The group to list service accounts for (required).")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat, "Format output as: text, json.")
	return cmd
}

func (o *options) complete(cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	o.group = group
	return nil
}

func (o *options) validate() error {
	if o.group == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--group' is not set")}
	}
	return nil
}

type ServiceAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type ServiceAccounts []ServiceAccount

func (o *options) run() error {
	var repoHost string
	if baseRepo, err := o.baseRepo(); err == nil {
		repoHost = baseRepo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	listOpts := &gitlab.ListServiceAccountsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	accounts, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.GroupServiceAccount, *gitlab.Response, error) {
		return client.Groups.ListServiceAccounts(o.group, listOpts, p)
	})
	if err != nil {
		return err
	}

	outputAccounts := make(ServiceAccounts, 0, len(accounts))
	for _, a := range accounts {
		outputAccounts = append(outputAccounts, ServiceAccount{
			ID:       strconv.FormatInt(a.ID, 10),
			Name:     a.Name,
			Username: a.UserName,
			Email:    a.Email,
		})
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(outputAccounts)
	}

	table := createTablePrinter(outputAccounts)
	o.io.LogInfof("%s", table.String())
	return nil
}
