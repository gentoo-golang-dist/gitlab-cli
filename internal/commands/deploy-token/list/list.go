package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

type options struct {
	gitlabClient func() (*gitlab.Client, error)
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)

	page         int
	perPage      int
	group        string
	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List deploy tokens for a project or group.",
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
		  $ glab deploy-token list
		  $ glab deploy-token list -g mygroup
		  $ glab deploy-token list --per-page 50 --page 2
		`),
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.page < 1 {
				return cmdutils.FlagError{Err: fmt.Errorf("--page must be >= 1")}
			}
			if opts.perPage < 1 {
				return cmdutils.FlagError{Err: fmt.Errorf("--per-page must be >= 1")}
			}
			return opts.run()
		},
	}

	cmd.Flags().IntVarP(&opts.page, "page", "p", 1, "Page number.")
	cmd.Flags().IntVarP(&opts.perPage, "per-page", "P", 30, "Number of items per page.")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "List deploy tokens for a group.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	var tokens []*gitlab.DeployToken

	if o.group != "" {
		groupOpts := &gitlab.ListGroupDeployTokensOptions{
			ListOptions: gitlab.ListOptions{
				Page:    int64(o.page),
				PerPage: int64(o.perPage),
			},
		}
		tokens, _, err = client.DeployTokens.ListGroupDeployTokens(o.group, groupOpts)
	} else {
		baseRepo, repoErr := o.baseRepo()
		if repoErr != nil {
			return repoErr
		}
		listOpts := &gitlab.ListProjectDeployTokensOptions{
			ListOptions: gitlab.ListOptions{
				Page:    int64(o.page),
				PerPage: int64(o.perPage),
			},
		}
		tokens, _, err = client.DeployTokens.ListProjectDeployTokens(baseRepo.FullName(), listOpts)
	}
	if err != nil {
		return cmdutils.WrapError(err, "failed to list deploy tokens")
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(tokens)
	}

	if len(tokens) == 0 {
		o.io.LogInfo("No deploy tokens found.\n")
		return nil
	}

	table := tableprinter.NewTablePrinter()
	table.AddRow("ID", "Name", "Username", "Scopes", "Expires At", "Revoked", "Expired")
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
