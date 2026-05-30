package create

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/resolve"
	"gitlab.com/gitlab-org/cli/internal/commands/token/expirationdate"
	"gitlab.com/gitlab-org/cli/internal/commands/token/tokenduration"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	tokenName      string
	serviceAccount string
	group          string
	description    string
	scopes         []string
	duration       tokenduration.TokenDuration
	expireAt       expirationdate.ExpirationDate
	outputFormat   string
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
		duration:  tokenduration.TokenDuration(30 * 24 * time.Hour),
	}

	cmd := &cobra.Command{
		Use:   "create <token-name> [flags]",
		Short: "Create a personal access token for a service account.",
		Args:  cobra.ExactArgs(1),
		Long: heredoc.Doc(`
			Create a new personal access token for a service account in a group.

			The expiration date of the token is calculated by adding the duration
			(default: 30 days) to the current date. You can specify a different
			duration using days (d), weeks (w), or hours (h), or provide an
			explicit end date.

			The token value is printed to stdout.
		`),
		Example: heredoc.Doc(`
		# Create a token with API scope
		glab service-account token create my-token --service-account my-bot --group my-group --scope api

		# Create a token with multiple scopes and 90 day lifetime
		glab service-account token create my-token --service-account my-bot --group my-group --scope api --scope read_registry --duration 90d

		# Create a token with explicit expiration date
		glab service-account token create my-token --service-account 12345 --group my-group --scope api --expires-at 2025-12-31`),
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.tokenName = args[0]

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
	cmd.Flags().StringVar(&opts.description, "description", "", "Description of the token.")
	cmd.Flags().StringSliceVarP(&opts.scopes, "scope", "S", []string{}, "Scopes for the token (required). Use multiple flags or comma-separated values.")
	cmd.Flags().VarP(&opts.duration, "duration", "D", "Sets the token lifetime. Accepts: days (30d), weeks (4w), or hours in multiples of 24 (24h). Maximum: 365d.")
	cmd.Flags().VarP(&opts.expireAt, "expires-at", "E", "Sets the token's expiration date, in YYYY-MM-DD format. If not specified, --duration is used.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat, "Format output as 'text' for the token value, 'json' for the full token object.")
	cmd.MarkFlagsMutuallyExclusive("expires-at", "duration")
	return cmd
}

func (o *options) complete(cmd *cobra.Command) error {
	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	o.group = group

	if time.Time(o.expireAt).IsZero() {
		o.expireAt = expirationdate.ExpirationDate(o.duration.CalculateExpirationDate())
	}

	return nil
}

func (o *options) validate() error {
	if o.group == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--group' is not set")}
	}
	if o.serviceAccount == "" {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--service-account' is not set")}
	}
	if len(o.scopes) == 0 {
		return cmdutils.FlagError{Err: fmt.Errorf("the required flag '--scope' is not set")}
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

	expirationDate := gitlab.ISOTime(o.expireAt)
	createOpts := &gitlab.CreateServiceAccountPersonalAccessTokenOptions{
		Name:      &o.tokenName,
		Scopes:    &o.scopes,
		ExpiresAt: &expirationDate,
	}
	if o.description != "" {
		createOpts.Description = &o.description
	}

	token, _, err := client.Groups.CreateServiceAccountPersonalAccessToken(o.group, saID, createOpts)
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		encoder.SetIndent("", "  ")
		return encoder.Encode(token)
	}

	_, err = fmt.Fprintf(o.io.StdOut, "%s\n", token.Token)
	return err
}
