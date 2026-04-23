package revoke

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/serviceaccount/resolve"
	"gitlab.com/gitlab-org/cli/internal/commands/token/filter"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	tokenID        int64
	tokenName      string
	serviceAccount string
	group          string
	outputFormat   string
}

func NewCmdRevoke(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "revoke <token-name|token-id> [flags]",
		Short:   "Revoke a personal access token for a service account.",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		Long: heredoc.Doc(`
			Revoke a personal access token for a service account in a group.

			If multiple tokens share the same name, specify the token ID
			to select the correct one.
		`),
		Example: heredoc.Doc(`
		# Revoke a token by name
		glab service-account token revoke my-token --service-account my-bot --group my-group

		# Revoke a token by ID
		glab service-account token revoke 12345 --service-account my-bot --group my-group`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
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
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat, "Format output as: text, json. 'text' provides the name and ID of the revoked token; 'json' outputs the token with metadata.")
	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	if tokenID, err := strconv.ParseInt(args[0], 10, 64); err != nil {
		o.tokenName = args[0]
	} else {
		o.tokenID = tokenID
	}

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

	activeState := "active"
	listOpts := &gitlab.ListServiceAccountPersonalAccessTokensOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
		State:       &activeState,
	}
	tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.PersonalAccessToken, *gitlab.Response, error) {
		return client.Groups.ListServiceAccountPersonalAccessTokens(o.group, saID, listOpts, p)
	})
	if err != nil {
		return err
	}

	tokens = filter.Filter(tokens, func(t *gitlab.PersonalAccessToken) bool {
		return t.Name == o.tokenName || t.ID == o.tokenID
	})

	var token *gitlab.PersonalAccessToken
	switch len(tokens) {
	case 1:
		token = tokens[0]
	case 0:
		return cmdutils.FlagError{Err: fmt.Errorf("no active token found matching %q", o.tokenIdentifier())}
	default:
		return cmdutils.FlagError{Err: fmt.Errorf("multiple tokens found matching %q; use the numeric ID instead", o.tokenIdentifier())}
	}

	_, err = client.Groups.RevokeServiceAccountPersonalAccessToken(o.group, saID, token.ID)
	if err != nil {
		return err
	}
	token.Revoked = true

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		return encoder.Encode(token)
	}

	_, err = fmt.Fprintf(o.io.StdOut, "Revoked token %s (ID: %d)\n", token.Name, token.ID)
	return err
}

func (o *options) tokenIdentifier() string {
	if o.tokenID != 0 {
		return strconv.FormatInt(o.tokenID, 10)
	}
	return o.tokenName
}
