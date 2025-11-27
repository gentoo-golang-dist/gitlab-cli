package revoke

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/token/filter"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	tokenID int64
	name    string

	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	group        string
	outputFormat string
}

func NewCmdRevoke(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "revoke <deploy-token-name|deploy-token-id>",
		Short:   "Revoke group or project tokens",
		Aliases: []string{"revoke", "rm"},
		Args:    cobra.RangeArgs(1, 1),
		Long: heredoc.Doc(`
			Revoke (delete) a group or project deploy token. If multiple tokens with the same name exist, you can specify
			the ID of the token.

			The output format can be either "JSON" or "text". The JSON output will show the meta information of the
			revoked token. The normal text output is a description of the revoked token name and ID.
		`),

		Example: heredoc.Doc(`
		# Revoke a project deploy token of the current project
		$ glab deploy-token revoke my-project-token

		# Revoke a project deploy token of a specific project
		$ glab deploy-token revoke --repo user/my-repo my-project-token

		# Revoke a group deploy token
		$ glab deploy-token revoke --group group/sub-group my-group-token
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
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Revoke group deploy token. Ignored if a repository argument is set.")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as: text, json. 'text' provides the name and ID of the revoked token; 'json' outputs the token with metadata.")
	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	if tokenID, err := strconv.ParseInt(args[0], 10, 64); err != nil {
		o.name = args[0]
	} else {
		o.tokenID = tokenID
	}
	if group, err := cmdutils.GroupOverride(cmd); err != nil {
		return err
	} else {
		o.group = group
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
		options := &gitlab.ListGroupDeployTokensOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListGroupDeployTokens(o.group, options, p)
		})
		if err != nil {
			return err
		}
		var token *gitlab.DeployToken
		tokens = filter.Filter(tokens, func(t *gitlab.DeployToken) bool {
			return (t.Name == o.name || t.ID == o.tokenID)
		})
		switch len(tokens) {
		case 1:
			token = tokens[0]
		case 0:
			return cmdutils.FlagError{Err: fmt.Errorf("no token found with the name '%v'.", o.name)}
		default:
			return cmdutils.FlagError{Err: fmt.Errorf("multiple tokens found with the name '%v'. Use the ID instead.", o.name)}
		}

		if _, err = client.DeployTokens.DeleteGroupDeployToken(o.group, token.ID); err != nil {
			return err
		}
		token.Revoked = true
		outputToken = token
		outputTokenValue = fmt.Sprintf("revoked %s %d", token.Name, token.ID)
	} else {
		repo, err := o.baseRepo()
		if err != nil {
			return err
		}
		options := &gitlab.ListProjectDeployTokensOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
		tokens, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.DeployToken, *gitlab.Response, error) {
			return client.DeployTokens.ListProjectDeployTokens(repo.FullName(), options, p)
		})
		if err != nil {
			return err
		}
		tokens = filter.Filter(tokens, func(t *gitlab.DeployToken) bool {
			return (t.Name == o.name || t.ID == o.tokenID)
		})
		var token *gitlab.DeployToken
		switch len(tokens) {
		case 1:
			token = tokens[0]
		case 0:
			return cmdutils.FlagError{Err: fmt.Errorf("no token found with the name '%v'.", o.name)}
		default:
			return cmdutils.FlagError{Err: fmt.Errorf("multiple tokens found with the name '%v'. Use the ID instead.", o.name)}
		}

		if _, err := client.DeployTokens.DeleteProjectDeployToken(repo.FullName(), token.ID); err != nil {
			return err
		}
		token.Revoked = true
		outputToken = token
		outputTokenValue = fmt.Sprintf("revoked %s %d", token.Name, token.ID)
	}

	if o.outputFormat == "json" {
		encoder := json.NewEncoder(o.io.StdOut)
		if err := encoder.Encode(outputToken); err != nil {
			return err
		}
	} else {
		if _, err := o.io.StdOut.Write([]byte(outputTokenValue)); err != nil {
			return err
		}
	}

	return nil
}
