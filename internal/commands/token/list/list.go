package list

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/token/accesslevel"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	user         string
	group        string
	outputFormat string
	listActive   bool
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List user, group, or project access tokens.",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		Long: heredoc.Doc(`
			List all tokens of a user, group, or project.

			The output contains the token's meta information, not the actual token value. The output format
			can be "JSON" or "text". The access level property is printed in human-readable form in the text
			output, but displays the integer value in JSON.

			Administrators can list tokens of other users.
		`),
		Example: heredoc.Doc(`
		# List the current project's access tokens
		$ glab token list
		$ glab token list --output json

		# List the project access tokens of a specific project
		$ glab token list --repo user/my-repo

		# List group access tokens
		$ glab token list --group group/sub-group

		# List my personal access tokens
		$ glab token list --user @me

		# Administrators only: list the personal access tokens of another user
		$ glab token list --user johndoe
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd); err != nil {
				return err
			}

			return opts.run()
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "List group access tokens. Ignored if a user or repository argument is set.")
	cmd.Flags().StringVarP(&opts.user, "user", "U", "", "List personal access tokens. Use @me for the current user.")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as: text, json. text provides a readable table, json outputs the tokens with metadata.")
	cmd.Flags().BoolVarP(&opts.listActive, "active", "a", false, "List only the active tokens.")
	cmd.MarkFlagsMutuallyExclusive("group", "user")

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

type Token struct {
	ID          int64
	Name        string
	Description string
	AccessLevel gitlab.AccessLevelValue
	Active      bool
	Revoked     bool
	CreatedAt   *time.Time
	ExpiresAt   *gitlab.ISOTime
	LastUsedAt  *time.Time
	Scopes      []string
}

type Tokens []Token

type FormattedToken struct {
	ID          string
	Name        string
	Description string
	AccessLevel string
	Active      string
	Revoked     string
	CreatedAt   string
	ExpiresAt   string
	LastUsedAt  string
	Scopes      string
}

func formatToken(token Token) FormattedToken {
	createdAtStr := ""
	if token.CreatedAt != nil {
		createdAtStr = token.CreatedAt.Format(time.RFC3339)
	}

	return FormattedToken{
		ID:          strconv.FormatInt(token.ID, 10),
		Name:        token.Name,
		Description: formatDescription(token.Description),
		AccessLevel: formatAccessLevel(token.AccessLevel),
		Active:      strconv.FormatBool(token.Active),
		Revoked:     strconv.FormatBool(token.Revoked),
		CreatedAt:   createdAtStr,
		ExpiresAt:   formatExpiresAt(token.ExpiresAt),
		LastUsedAt:  formatLastUsedAt(token.LastUsedAt),
		Scopes:      strings.Join(token.Scopes, ","),
	}
}

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

func formatAccessLevel(accessLevel gitlab.AccessLevelValue) string {
	level := accesslevel.AccessLevel{Value: accessLevel}
	levelStr := level.String()

	if levelStr == "no" {
		return "-"
	}

	return levelStr
}

func formatExpiresAt(expiresAt *gitlab.ISOTime) string {
	if expiresAt == nil {
		return "-"
	}
	return expiresAt.String()
}

func (o *options) run() error {
	// NOTE: this command can not only be used for projects,
	// so we have to manually check for the base repo, if it doesn't exist,
	// we bootstrap the client with the default hostname.
	var repoHost string
	if baseRepo, err := o.baseRepo(); err == nil {
		repoHost = baseRepo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	var apiTokens any
	var tokens Tokens

	switch {
	case o.user != "":
		user, err := api.UserByName(client, o.user)
		if err != nil {
			return err
		}
		options := &gitlab.ListPersonalAccessTokensOptions{
			UserID: &user.ID,
		}
		apiTokens, tokens, err = o.filterPersonalAccessTokens(client, options)
		if err != nil {
			return err
		}
	case o.group != "":
		tokens, apiTokens, err = o.filterGroupAccessTokens(client)
		if err != nil {
			return err
		}
	default:
		repo, err := o.baseRepo()
		if err != nil {
			return err
		}

		opts := &gitlab.ListProjectAccessTokensOptions{}
		tokens, apiTokens, err = o.filterProjectAccessTokens(client, repo.FullName(), opts)
		if err != nil {
			return err
		}
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		if err := encoder.Encode(apiTokens); err != nil {
			return err
		}
	} else {
		formattedTokens := make([]FormattedToken, 0, len(tokens))
		for _, token := range tokens {
			formattedTokens = append(formattedTokens, formatToken(token))
		}
		table := createTablePrinter(formattedTokens)
		o.io.LogInfof("%s", table.String())
	}
	return nil
}

func (o *options) filterPersonalAccessTokens(client *gitlab.Client, options *gitlab.ListPersonalAccessTokensOptions) (any, Tokens, error) {
	apiTokensList, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.PersonalAccessToken, *gitlab.Response, error) {
		return client.PersonalAccessTokens.ListPersonalAccessTokens(options, p)
	})
	if err != nil {
		return nil, nil, err
	}

	filteredTokens := make([]*gitlab.PersonalAccessToken, 0, len(apiTokensList))
	tokens := make(Tokens, 0, len(apiTokensList))

	for _, apiToken := range apiTokensList {
		if !o.listActive || apiToken.Active {
			filteredTokens = append(filteredTokens, apiToken)
			tokens = append(tokens, Token{
				ID:          apiToken.ID,
				Name:        apiToken.Name,
				Description: apiToken.Description,
				AccessLevel: 0, // PersonalAccessTokens don't have AccessLevel
				Active:      apiToken.Active,
				Revoked:     apiToken.Revoked,
				CreatedAt:   apiToken.CreatedAt,
				ExpiresAt:   apiToken.ExpiresAt,
				LastUsedAt:  apiToken.LastUsedAt,
				Scopes:      apiToken.Scopes,
			})
		}
	}

	return filteredTokens, tokens, nil
}

func (o *options) filterGroupAccessTokens(client *gitlab.Client) (Tokens, any, error) {
	options := &gitlab.ListGroupAccessTokensOptions{}
	apiTokensList, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.GroupAccessToken, *gitlab.Response, error) {
		return client.GroupAccessTokens.ListGroupAccessTokens(o.group, options, p)
	})
	if err != nil {
		return nil, nil, err
	}

	filteredTokens := make([]*gitlab.GroupAccessToken, 0, len(apiTokensList))
	tokens := make(Tokens, 0, len(apiTokensList))

	for _, apiToken := range apiTokensList {
		if !o.listActive || apiToken.Active {
			filteredTokens = append(filteredTokens, apiToken)
			tokens = append(tokens, Token{
				ID:          apiToken.ID,
				Name:        apiToken.Name,
				Description: apiToken.Description,
				AccessLevel: apiToken.AccessLevel,
				Active:      apiToken.Active,
				Revoked:     apiToken.Revoked,
				CreatedAt:   apiToken.CreatedAt,
				ExpiresAt:   apiToken.ExpiresAt,
				LastUsedAt:  apiToken.LastUsedAt,
				Scopes:      apiToken.Scopes,
			})
		}
	}

	return tokens, filteredTokens, nil
}

func (o *options) filterProjectAccessTokens(client *gitlab.Client, projectName string, opts *gitlab.ListProjectAccessTokensOptions) (Tokens, any, error) {
	apiTokensList, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.ProjectAccessToken, *gitlab.Response, error) {
		return client.ProjectAccessTokens.ListProjectAccessTokens(projectName, opts, p)
	})
	if err != nil {
		return nil, nil, err
	}

	filteredTokens := make([]*gitlab.ProjectAccessToken, 0, len(apiTokensList))
	tokens := make(Tokens, 0, len(apiTokensList))

	for _, apiToken := range apiTokensList {
		if !o.listActive || apiToken.Active {
			filteredTokens = append(filteredTokens, apiToken)
			tokens = append(tokens, Token{
				ID:          apiToken.ID,
				Name:        apiToken.Name,
				Description: apiToken.Description,
				AccessLevel: apiToken.AccessLevel,
				Active:      apiToken.Active,
				Revoked:     apiToken.Revoked,
				CreatedAt:   apiToken.CreatedAt,
				ExpiresAt:   apiToken.ExpiresAt,
				LastUsedAt:  apiToken.LastUsedAt,
				Scopes:      apiToken.Scopes,
			})
		}
	}

	return tokens, filteredTokens, nil
}
