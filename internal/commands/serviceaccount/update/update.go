package update

import (
	"encoding/json"
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
	name           string
	username       string
	outputFormat   string
}

func NewCmdUpdate(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Update a service account in a group.",
		Long: heredoc.Doc(`
			Update a service account's name or username.

			This API endpoint works on top-level groups only. It does not
			work on subgroups.
		`),
		Example: heredoc.Doc(`
		# Update a service account's name
		glab service-account update --service-account my-bot --group my-group --name "New Name"

		# Update a service account's username by numeric ID
		glab service-account update --service-account 12345 --group my-group --username new-bot-name`),
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
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
	cmd.Flags().StringVar(&opts.name, "name", "", "New name for the service account.")
	cmd.Flags().StringVar(&opts.username, "username", "", "New username for the service account.")
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
	if o.serviceAccount == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--service-account' is not set")}
	}
	if o.name == "" && o.username == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("at least one of '--name' or '--username' must be set")}
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

	updateOpts := &gitlab.UpdateServiceAccountOptions{}
	if o.name != "" {
		updateOpts.Name = &o.name
	}
	if o.username != "" {
		updateOpts.Username = &o.username
	}

	sa, _, err := client.Groups.UpdateServiceAccount(o.group, saID, updateOpts)
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		encoder.SetIndent("", "  ")
		return encoder.Encode(sa)
	}

	_, err = fmt.Fprintf(o.io.StdOut, "Updated service account %d (%s)\n", sa.ID, sa.UserName)
	return err
}
