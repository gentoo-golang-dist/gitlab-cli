package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/resolve"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	serviceAccount string
	group          string
}

func NewCmdDelete(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "delete [flags]",
		Short:   "Delete a service account from a group.",
		Aliases: []string{"rm"},
		Long: heredoc.Doc(`
			Delete a service account from a group.

			This API endpoint works on top-level groups only. It does not
			work on subgroups.
		`),
		Example: heredoc.Doc(`
		# Delete a service account by name
		glab service-account delete --service-account my-bot --group my-group

		# Delete a service account by numeric ID
		glab service-account delete --service-account 12345 --group my-group`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
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
	cmd.Flags().StringVar(&opts.serviceAccount, "service-account", "", "The service account name or numeric ID (required).")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "The group the service account belongs to (required).")
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
	if o.serviceAccount == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--service-account' is not set")}
	}
	return nil
}

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

	saID, err := resolve.ServiceAccountID(client, o.group, o.serviceAccount)
	if err != nil {
		return err
	}

	_, err = client.Groups.DeleteServiceAccount(o.group, saID, &gitlab.DeleteServiceAccountOptions{})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(o.io.StdOut, "Deleted service account %d from group %s\n", saID, o.group)
	return err
}
