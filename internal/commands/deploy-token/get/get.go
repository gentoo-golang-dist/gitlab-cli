package get

import (
	"fmt"
	"strconv"

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

	tokenID      int
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
		Use:   "get <token-id> [flags]",
		Short: "Get a deploy token by ID.",
		Example: heredoc.Doc(`
		  $ glab deploy-token get 42
		  $ glab deploy-token get 42 -g mygroup
		  $ glab deploy-token get 42 --output json
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}
			return opts.run()
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	fl := cmd.Flags()
	fl.StringVarP(&opts.group, "group", "g", "", "Get deploy token for a group. Ignored if -R/--repo is set.")

	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return cmdutils.FlagError{Err: fmt.Errorf("invalid token ID %q: %w", args[0], err)}
	}
	o.tokenID = id

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

	var token *gitlab.DeployToken

	switch {
	case o.group != "":
		token, _, err = client.DeployTokens.GetGroupDeployToken(o.group, int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to get group deploy token")
		}
	default:
		if repoErr != nil {
			return repoErr
		}
		token, _, err = client.DeployTokens.GetProjectDeployToken(repo.FullName(), int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to get project deploy token")
		}
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(token)
	default:
		return o.printDetails(token)
	}
}

func (o *options) printDetails(token *gitlab.DeployToken) error {
	c := o.io.Color()
	table := tableprinter.NewTablePrinter()
	table.AddRow(c.Bold("ID"), token.ID)
	table.AddRow(c.Bold("Name"), token.Name)
	table.AddRow(c.Bold("Username"), token.Username)
	table.AddRow(c.Bold("Scopes"), fmt.Sprintf("%v", token.Scopes))

	if token.ExpiresAt != nil {
		table.AddRow(c.Bold("Expires At"), token.ExpiresAt.String())
	} else {
		table.AddRow(c.Bold("Expires At"), "Never")
	}

	var status string
	switch {
	case token.Revoked:
		status = c.Yellow("Revoked")
	case token.Expired:
		status = c.Red("Expired")
	default:
		status = c.Green("Active")
	}
	table.AddRow(c.Bold("Status"), status)

	fmt.Fprint(o.io.StdOut, table.Render())
	return nil
}
