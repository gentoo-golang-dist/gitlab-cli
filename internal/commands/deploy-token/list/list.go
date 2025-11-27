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
		Short:   "List group or project access tokens.",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		Long: heredoc.Doc(`
			List all deploy tokens of a group or project.

			The output contains the deploy token's meta information, not the actual deploy token value. The output format
			can be "JSON" or "text". The access level property is printed in human-readable form in the text
			output, but displays the integer value in JSON.
		`),
		Example: heredoc.Doc(`
		# List the current project's deploy tokens
		$ glab deploy-token list
		$ glab deploy-token list --output json

		# List the project deploy tokens of a specific project
		$ glab deploy-token list --repo user/my-repo

		# List group deploy tokens
		$ glab deploy-token list --group group/sub-group
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
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "List group deploy tokens. Ignored if a repository argument is set.")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as: text, json. text provides a readable table, json outputs the tokens with metadata.")
	cmd.Flags().BoolVarP(&opts.listActive, "active", "a", false, "List only the active deploy tokens.")

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

type DeployToken struct {
	ID        string
	Name      string
	Username  string
	ExpiresAt string
	Revoked   string
	Expired   string
	Scopes    string
}

type DeployTokens []DeployToken

func formatExpiresAt(expiresAt *time.Time) string {
	if expiresAt == nil {
		return "-"
	}
	return expiresAt.Format(time.RFC3339)
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

	var apiTokens any
	var outputTokens DeployTokens
	switch {
	case o.group != "":
		options := &gitlab.ListGroupDeployTokensOptions{}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListGroupDeployTokens(o.group, options, p)
		})
		if err != nil {
			return err
		}
		apiTokens = tokens
		outputTokens = make([]DeployToken, 0, len(tokens))
		for _, token := range tokens {
			if !o.listActive {
				outputTokens = append(outputTokens, DeployToken{
					ID:        strconv.FormatInt(token.ID, 10),
					Name:      token.Name,
					Username:  token.Username,
					ExpiresAt: formatExpiresAt(token.ExpiresAt),
					Revoked:   strconv.FormatBool(token.Revoked),
					Expired:   strconv.FormatBool(token.Expired),
					Scopes:    strings.Join(token.Scopes, ","),
				})
			}
		}
	default:
		repo, err := o.baseRepo()
		if err != nil {
			return err
		}

		opts := &gitlab.ListProjectDeployTokensOptions{}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListProjectDeployTokens(repo.FullName(), opts, p)
		})
		if err != nil {
			return err
		}
		apiTokens = tokens
		outputTokens = make([]DeployToken, 0, len(tokens))
		for _, token := range tokens {
			if !o.listActive {
				outputTokens = append(outputTokens, DeployToken{
					ID:        strconv.FormatInt(token.ID, 10),
					Name:      token.Name,
					Username:  token.Username,
					ExpiresAt: formatExpiresAt(token.ExpiresAt),
					Revoked:   strconv.FormatBool(token.Revoked),
					Expired:   strconv.FormatBool(token.Expired),
					Scopes:    strings.Join(token.Scopes, ","),
				})
			}
		}
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		if err := encoder.Encode(apiTokens); err != nil {
			return err
		}
	} else {
		table := createTablePrinter(outputTokens)
		o.io.LogInfof("%s", table.String())
	}
	return nil
}
