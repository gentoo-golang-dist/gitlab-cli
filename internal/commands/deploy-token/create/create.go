package create

import (
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	gitlabClient func() (*gitlab.Client, error)
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)

	name      string
	scopes    []string
	expiresAt string
	username  string
	group     string
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a deploy token for a project or group.",
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
		  $ glab deploy-token create --name "CI Token" --scopes read_repository
		  $ glab deploy-token create --name "Registry" --scopes read_registry,write_registry --expires-at 2025-12-31
		  $ glab deploy-token create --name "Group Token" --scopes read_repository -g mygroup
		`),
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.name == "" {
				return cmdutils.FlagError{Err: fmt.Errorf("--name is required")}
			}
			if len(opts.scopes) == 0 {
				return cmdutils.FlagError{Err: fmt.Errorf("--scopes is required")}
			}
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.name, "name", "n", "", "Name of the deploy token.")
	cmd.Flags().StringSliceVarP(&opts.scopes, "scopes", "s", nil, "Scopes for the deploy token. Options: read_repository, read_registry, write_registry, read_package_registry, write_package_registry.")
	cmd.Flags().StringVarP(&opts.expiresAt, "expires-at", "e", "", "Expiration date of the deploy token (ISO 8601 format, e.g. 2025-12-31).")
	cmd.Flags().StringVarP(&opts.username, "username", "u", "", "Username for the deploy token. Defaults to gitlab+deploy-token-{n}.")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Create deploy token for a group.")

	return cmd
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	c := o.io.Color()

	var expiresAt *time.Time
	if o.expiresAt != "" {
		t, err := time.Parse("2006-01-02", o.expiresAt)
		if err != nil {
			return fmt.Errorf("invalid date format for --expires-at: %w. Use YYYY-MM-DD", err)
		}
		expiresAt = &t
	}

	var token *gitlab.DeployToken

	if o.group != "" {
		createOpts := &gitlab.CreateGroupDeployTokenOptions{
			Name:   gitlab.Ptr(o.name),
			Scopes: &o.scopes,
		}
		if expiresAt != nil {
			createOpts.ExpiresAt = expiresAt
		}
		if o.username != "" {
			createOpts.Username = gitlab.Ptr(o.username)
		}
		token, _, err = client.DeployTokens.CreateGroupDeployToken(o.group, createOpts)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create group deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Created deploy token for group %s\n", c.GreenCheck(), o.group)
	} else {
		baseRepo, repoErr := o.baseRepo()
		if repoErr != nil {
			return repoErr
		}
		createOpts := &gitlab.CreateProjectDeployTokenOptions{
			Name:   gitlab.Ptr(o.name),
			Scopes: &o.scopes,
		}
		if expiresAt != nil {
			createOpts.ExpiresAt = expiresAt
		}
		if o.username != "" {
			createOpts.Username = gitlab.Ptr(o.username)
		}
		token, _, err = client.DeployTokens.CreateProjectDeployToken(baseRepo.FullName(), createOpts)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create project deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Created deploy token for %s\n", c.GreenCheck(), baseRepo.FullName())
	}

	fmt.Fprintf(o.io.StdOut, "Token:\t\t%s\n", token.Token)
	fmt.Fprintf(o.io.StdOut, "Name:\t\t%s\n", token.Name)
	fmt.Fprintf(o.io.StdOut, "Username:\t%s\n", token.Username)
	fmt.Fprintf(o.io.StdOut, "Scopes:\t\t%v\n", token.Scopes)
	if token.ExpiresAt != nil {
		fmt.Fprintf(o.io.StdOut, "Expires At:\t%s\n", token.ExpiresAt.String())
	}
	fmt.Fprintf(o.io.StdOut, "\n%s Make sure to save the token — you won't be able to see it again.\n", c.WarnIcon())

	return nil
}
