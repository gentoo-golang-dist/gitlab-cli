package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

type options struct {
	io        *iostreams.IOStreams
	apiClient func(repoHost string) (*api.Client, error)
	baseRepo  func() (glrepo.Interface, error)

	page         int64
	perPage      int64
	group        string
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List deploy tokens for a project or group.",
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
		  $ glab deploy-token list
		  $ glab deploy-token list -g mygroup
		  $ glab deploy-token list --per-page 50 --page 2
		  $ glab deploy-token list --output json
		`),
		Args: cobra.NoArgs,
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
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	fl := cmd.Flags()
	fl.Int64VarP(&opts.page, "page", "p", 1, "Page number.")
	fl.Int64VarP(&opts.perPage, "per-page", "P", api.DefaultListLimit, "Number of items to list per page.")
	fl.StringVarP(&opts.group, "group", "g", "", "List deploy tokens for a group. Ignored if -R/--repo is set.")

	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (o *options) complete(cmd *cobra.Command) error {
	if o.page < 1 {
		return cmdutils.FlagError{Err: fmt.Errorf("--page must be >= 1")}
	}
	if o.perPage < 1 {
		return cmdutils.FlagError{Err: fmt.Errorf("--per-page must be >= 1")}
	}

	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	o.group = group

	return nil
}

func (o *options) run() error {
	repo, repoErr := o.baseRepo()
	var repoHost string
	if repoErr == nil {
		repoHost = repo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	listOptsBase := gitlab.ListOptions{
		Page:    o.page,
		PerPage: o.perPage,
	}

	var tokens []*gitlab.DeployToken

	switch {
	case o.group != "":
		tokens, _, err = client.DeployTokens.ListGroupDeployTokens(o.group, &gitlab.ListGroupDeployTokensOptions{
			ListOptions: listOptsBase,
		})
		if err != nil {
			return cmdutils.WrapError(err, "failed to list group deploy tokens")
		}
	default:
		if repoErr != nil {
			return repoErr
		}
		tokens, _, err = client.DeployTokens.ListProjectDeployTokens(repo.FullName(), &gitlab.ListProjectDeployTokensOptions{
			ListOptions: listOptsBase,
		})
		if err != nil {
			return cmdutils.WrapError(err, "failed to list project deploy tokens")
		}
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(tokens)
	default:
		return o.printTable(tokens)
	}
}

func (o *options) printTable(tokens []*gitlab.DeployToken) error {
	if len(tokens) == 0 {
		o.io.LogInfo("No deploy tokens found.\n")
		return nil
	}

	c := o.io.Color()
	table := tableprinter.NewTablePrinter()
	table.AddRow(c.Bold("ID"), c.Bold("Name"), c.Bold("Username"), c.Bold("Scopes"), c.Bold("Expires At"), c.Bold("Revoked"), c.Bold("Expired"))
	for _, t := range tokens {
		expires := "Never"
		if t.ExpiresAt != nil {
			expires = t.ExpiresAt.String()
		}
		table.AddRow(t.ID, t.Name, t.Username, fmt.Sprintf("%v", t.Scopes), expires, t.Revoked, t.Expired)
	}
	o.io.LogInfo(table.String())

	return nil
}
