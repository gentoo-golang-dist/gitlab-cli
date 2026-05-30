package list

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	outputFormat   string
	listActive     bool
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List personal access tokens for a service account.",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		Long: heredoc.Doc(`
			List all personal access tokens for a service account in a group.

			The output contains the token's metadata, not the actual token value.
		`),
		Example: heredoc.Doc(`
		# List tokens for a service account
		glab service-account token list --service-account my-bot --group my-group

		# List only active tokens as JSON
		glab service-account token list --service-account my-bot --group my-group --active --output json`),
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
	cmd.Flags().StringVar(&opts.serviceAccount, "service-account", "", "The service account name or numeric ID (required).")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "The group the service account belongs to (required).")
	cmd.Flags().BoolVarP(&opts.listActive, "active", "a", false, "List only active tokens.")
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
	return nil
}

type Token struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      string `json:"active"`
	Revoked     string `json:"revoked"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
	LastUsedAt  string `json:"last_used_at"`
	Scopes      string `json:"scopes"`
}

type Tokens []Token

func formatLastUsedAt(lastUsedAt *time.Time) string {
	if lastUsedAt == nil {
		return "-"
	}
	return lastUsedAt.Format(time.RFC3339)
}

func formatDescription(description string) string {
	if description == "" {
		return "-"
	}
	return description
}

func formatExpiresAt(expiresAt *gitlab.ISOTime) string {
	if expiresAt == nil {
		return "-"
	}
	return expiresAt.String()
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

	listOpts := &gitlab.ListServiceAccountPersonalAccessTokensOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.PersonalAccessToken, *gitlab.Response, error) {
		return client.Groups.ListServiceAccountPersonalAccessTokens(o.group, saID, listOpts, p)
	})
	if err != nil {
		return err
	}

	outputTokens := make(Tokens, 0, len(tokens))
	for _, t := range tokens {
		if !o.listActive || t.Active {
			outputTokens = append(outputTokens, Token{
				ID:          strconv.FormatInt(t.ID, 10),
				Name:        t.Name,
				Description: formatDescription(t.Description),
				Active:      strconv.FormatBool(t.Active),
				Revoked:     strconv.FormatBool(t.Revoked),
				CreatedAt:   t.CreatedAt.Format(time.RFC3339),
				ExpiresAt:   formatExpiresAt(t.ExpiresAt),
				LastUsedAt:  formatLastUsedAt(t.LastUsedAt),
				Scopes:      strings.Join(t.Scopes, ","),
			})
		}
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(outputTokens)
	}

	table := createTablePrinter(outputTokens)
	o.io.LogInfof("%s", table.String())
	return nil
}
