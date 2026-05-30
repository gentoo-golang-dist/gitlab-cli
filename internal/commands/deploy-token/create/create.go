package create

import (
	"fmt"
	"strings"
	"time"

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

	name         string
	scopes       []string
	expiresAt    string
	expiresAtPt  *time.Time
	username     string
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
		Use:     "create <name> [flags]",
		Short:   "Create a deploy token for a project or group.",
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
		  $ glab deploy-token create "CI Token" --scopes read_repository
		  $ glab deploy-token create "Registry" --scopes read_registry,write_registry --expires-at 2025-12-31
		  $ glab deploy-token create "Group Token" --scopes read_repository -g mygroup
		  $ glab deploy-token create "CI Token" --scopes read_repository --output json
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
	fl.StringSliceVarP(&opts.scopes, "scopes", "s", nil, "Scopes for the deploy token. Options: read_repository, read_registry, write_registry, read_package_registry, write_package_registry.")
	fl.StringVarP(&opts.expiresAt, "expires-at", "e", "", "Expiration date of the deploy token (YYYY-MM-DD format, e.g. 2025-12-31).")
	fl.StringVarP(&opts.username, "username", "u", "", "Username for the deploy token. Defaults to gitlab+deploy-token-{n}.")
	fl.StringVarP(&opts.group, "group", "g", "", "Create deploy token for a group. Ignored if -R/--repo is set.")

	_ = cmd.MarkFlagRequired("scopes")
	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	o.name = args[0]

	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	o.group = group

	if o.expiresAt != "" {
		t, err := time.Parse("2006-01-02", o.expiresAt)
		if err != nil {
			return cmdutils.FlagError{Err: fmt.Errorf("invalid date format for --expires-at: %w. Use YYYY-MM-DD", err)}
		}
		o.expiresAtPt = &t
	}

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

	c := o.io.Color()

	var token *gitlab.DeployToken

	switch {
	case o.group != "":
		createOpts := &gitlab.CreateGroupDeployTokenOptions{
			Name:   gitlab.Ptr(o.name),
			Scopes: gitlab.Ptr(o.scopes),
		}
		if o.expiresAtPt != nil {
			createOpts.ExpiresAt = o.expiresAtPt
		}
		if o.username != "" {
			createOpts.Username = gitlab.Ptr(o.username)
		}
		token, _, err = client.DeployTokens.CreateGroupDeployToken(o.group, createOpts)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create group deploy token")
		}
	default:
		if repoErr != nil {
			return repoErr
		}
		createOpts := &gitlab.CreateProjectDeployTokenOptions{
			Name:   gitlab.Ptr(o.name),
			Scopes: &o.scopes,
		}
		if o.expiresAtPt != nil {
			createOpts.ExpiresAt = o.expiresAtPt
		}
		if o.username != "" {
			createOpts.Username = gitlab.Ptr(o.username)
		}
		token, _, err = client.DeployTokens.CreateProjectDeployToken(repo.FullName(), createOpts)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create project deploy token")
		}
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(token)
	default:
		if o.group != "" {
			fmt.Fprintf(o.io.StdOut, "%s Created deploy token for group %s\n", c.GreenCheck(), o.group)
		} else {
			fmt.Fprintf(o.io.StdOut, "%s Created deploy token for %s\n", c.GreenCheck(), repo.FullName())
		}
		return o.printDetails(token, c)
	}
}

func (o *options) printDetails(token *gitlab.DeployToken, c *iostreams.ColorPalette) error {
	table := tableprinter.NewTablePrinter()
	table.AddRow(c.Bold("Token"), token.Token)
	table.AddRow(c.Bold("Name"), token.Name)
	table.AddRow(c.Bold("Username"), token.Username)
	table.AddRow(c.Bold("Scopes"), strings.Join(token.Scopes, ", "))

	expiresAt := "never"
	if token.ExpiresAt != nil {
		expiresAt = token.ExpiresAt.Format("2006-01-02")
	}
	table.AddRow(c.Bold("Expires At"), expiresAt)

	fmt.Fprint(o.io.StdOut, table.Render())
	fmt.Fprintf(o.io.StdOut, "\n%s Make sure to save the token — you won't be able to see it again.\n", c.WarnIcon())
	return nil
}
