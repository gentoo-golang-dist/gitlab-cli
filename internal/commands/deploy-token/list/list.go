package list

import (
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

type options struct {
	gitlabClient func() (*gitlab.Client, error)
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)

	// Pagination
	page    int
	perPage int

	// Filters
	active bool

	// Group mode
	group string
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deploy tokens for a project or group.",
		Long:  "",
		Example: heredoc.Doc(`
		  # List deploy tokens for the current project
		  $ glab deploy-token list

		  # List deploy tokens for a specific group
		  $ glab deploy-token list --group my-group

		  # List only active deploy tokens
		  $ glab deploy-token list --active
		`),
		Args: cobra.MaximumNArgs(0),
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

	cmd.Flags().BoolVarP(&opts.active, "active", "a", false, "Limit to active deploy tokens only.")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 1, "Page number.")
	cmd.Flags().IntVarP(&opts.perPage, "per-page", "P", 30, "Number of items to list per page.")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "List deploy tokens for a group instead of a project.")

	return cmd
}

func (o *options) complete(cmd *cobra.Command) error {
	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	table := tableprinter.NewTablePrinter()
	isTTy := o.io.IsOutputTTY()

	if o.group != "" {
		// List group deploy tokens
		listOptions := &gitlab.ListGroupDeployTokensOptions{
			ListOptions: gitlab.ListOptions{
				Page:    int64(o.page),
				PerPage: int64(o.perPage),
			},
		}

		tokens, _, err := client.DeployTokens.ListGroupDeployTokens(o.group, listOptions)
		if err != nil {
			return cmdutils.WrapError(err, "failed to get deploy tokens for group.")
		}

		if len(tokens) > 0 {
			table.AddRow("ID", "Name", "Username", "Scopes", "Expires At", "Revoked", "Expired")
		}

		for _, token := range tokens {
			if o.active && (token.Revoked || token.Expired) {
				continue
			}
			expiresAt := ""
			if token.ExpiresAt != nil {
				if isTTy {
					expiresAt = utils.TimeToPrettyTimeAgo(time.Time(*token.ExpiresAt))
				} else {
					expiresAt = token.ExpiresAt.String()
				}
			}
			table.AddRow(token.ID, token.Name, token.Username, token.Scopes, expiresAt, token.Revoked, token.Expired)
		}
	} else {
		// List project deploy tokens
		baseRepo, err := o.baseRepo()
		if err != nil {
			return err
		}

		listOptions := &gitlab.ListProjectDeployTokensOptions{
			ListOptions: gitlab.ListOptions{
				Page:    int64(o.page),
				PerPage: int64(o.perPage),
			},
		}

		tokens, _, err := client.DeployTokens.ListProjectDeployTokens(baseRepo.FullName(), listOptions)
		if err != nil {
			return cmdutils.WrapError(err, "failed to get deploy tokens for project.")
		}

		if len(tokens) > 0 {
			table.AddRow("ID", "Name", "Username", "Scopes", "Expires At", "Revoked", "Expired")
		}

		for _, token := range tokens {
			if o.active && (token.Revoked || token.Expired) {
				continue
			}
			expiresAt := ""
			if token.ExpiresAt != nil {
				if isTTy {
					expiresAt = utils.TimeToPrettyTimeAgo(time.Time(*token.ExpiresAt))
				} else {
					expiresAt = token.ExpiresAt.String()
				}
			}
			table.AddRow(token.ID, token.Name, token.Username, token.Scopes, expiresAt, token.Revoked, token.Expired)
		}
	}

	o.io.LogInfo(table.String())

	return nil
}
