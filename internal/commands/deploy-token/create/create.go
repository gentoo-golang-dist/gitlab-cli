package create

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/expirationdate"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/filter"
	"gitlab.com/gitlab-org/cli/internal/commands/deploy-token/tokenduration"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	name         string
	username     string
	group        string
	scopes       []string
	duration     tokenduration.TokenDuration
	expiresAt    expirationdate.ExpirationDate
	outputFormat string
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"create", "new"},
		Args:    cobra.RangeArgs(1, 1),
		Short:   "Creates group or project deploy tokens.",
		Long: heredoc.Doc(`
		Creates a new deploy token for a group or project. Defaults to a
		project deploy token, unless a group name is specified.

		The expiration date of the deploy token is calculated by adding the duration
		(default: 30 days) to the current date, with expiration occurring at midnight
		UTC on the calculated date. You can specify a different duration using days (d),
		weeks (w), or hours (h), or provide an explicit end date.

		The name of the deploy token must be unique. The token is printed to stdout.
		`),
		Example: heredoc.Doc(`
		# Create project deploy token for current project
		$ glab deploy-token create --scope read_repository --scope read_registry my-project-token

		# Create project deploy token with 7 day lifetime
		$ glab deploy-token create --repo user/my-repo --scope api my-project-token --duration 7d

		# Create a group deploy token expiring at 2026-05-04
		$ glab deploy-token create --group group/sub-group --scope api my-group-token --expires-at 2026-05-04
		`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}

			return opts.run()
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Create a group deploy token. Ignored if a user or repository argument is set.")
	cmd.Flags().StringVarP(&opts.username, "username", "U", "", "Sets the deploy token's username.")
	cmd.Flags().VarP(&opts.duration, "duration", "D", "Sets the deploy token lifetime in days. Accepts: days (30d), weeks (4w), or hours in multiples of 24 (24h, 168h, 720h). Maximum: 365d. The token expires at midnight UTC on the calculated date.")
	cmd.Flags().VarP(&opts.expiresAt, "expires-at", "E", "Sets the deploy token's expiration date and time, in YYYY-MM-DD format. If not specified, --duration is used.")
	cmd.Flags().StringSliceVarP(&opts.scopes, "scope", "S", []string{"read_repository"}, "Scopes for the token. For a list, see https://docs.gitlab.com/user/profile/personal_access_tokens/#personal-access-token-scopes.")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as 'text' for the token value, 'json' for the actual API token structure.")
	cmd.MarkFlagsMutuallyExclusive("expires-at", "duration")
	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	o.name = args[0]

	if group, err := cmdutils.GroupOverride(cmd); err != nil {
		return err
	} else {
		o.group = group
	}

	if time.Time(o.expiresAt).IsZero() && o.duration.Duration().Nanoseconds() != 0 {
		fmt.Printf("Calculating expiration date using duration: %s\n", o.duration.String())
		o.expiresAt = expirationdate.ExpirationDate(o.duration.CalculateExpirationDate())
	}

	return nil
}

func (o *options) run() error {
	// NOTE: this command can not only be used for projects,
	// so we have to manually check for the base repo, it it doesn't exist,
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

	var outputToken any
	var outputTokenValue string

	if o.group != "" {
		listOptions := &gitlab.ListGroupDeployTokensOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListGroupDeployTokens(o.group, listOptions, p)
		})
		if err != nil {
			return err
		}
		tokens = filter.Filter(tokens, func(t *gitlab.DeployToken) bool {
			return t.Name == o.name
		})
		if len(tokens) > 0 {
			return cmdutils.FlagError{Err: fmt.Errorf("a group deploy token with the name %s already exists.", o.name)}
		}

		options := &gitlab.CreateGroupDeployTokenOptions{
			Name:     &o.name,
			Username: &o.username,
			Scopes:   &o.scopes,
		}
		if !time.Time(o.expiresAt).IsZero() {
			options.ExpiresAt = (*time.Time)(&o.expiresAt)
		}

		token, _, err := client.DeployTokens.CreateGroupDeployToken(o.group, options)
		if err != nil {
			return err
		}
		outputToken = token
		outputTokenValue = token.Token

	} else {
		repo, err := o.baseRepo()
		if err != nil {
			return err
		}
		listOptions := &gitlab.ListProjectDeployTokensOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListProjectDeployTokens(repo.FullName(), listOptions, p)
		})
		if err != nil {
			return err
		}
		tokens = filter.Filter(tokens, func(t *gitlab.DeployToken) bool {
			return t.Name == o.name
		})

		if len(tokens) > 0 {
			return cmdutils.FlagError{Err: fmt.Errorf("a project deploy token with name %s already exists.", o.name)}
		}

		options := &gitlab.CreateProjectDeployTokenOptions{
			Name:     &o.name,
			Username: &o.username,
			Scopes:   &o.scopes,
		}
		if !time.Time(o.expiresAt).IsZero() {
			options.ExpiresAt = (*time.Time)(&o.expiresAt)
		}

		token, _, err := client.DeployTokens.CreateProjectDeployToken(repo.FullName(), options)
		if err != nil {
			return err
		}
		outputToken = token
		outputTokenValue = token.Token
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		encoder.SetIndent("  ", "  ")
		if err := encoder.Encode(outputToken); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(o.io.StdOut, "%s\n", outputTokenValue); err != nil {
			return err
		}
	}

	return nil
}
