package get

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

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

	tokenID int64
	group   string
}

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:   "get <token-id>",
		Short: "Get a single deploy token by ID.",
		Long:  ``,
		Example: heredoc.Doc(`
			# Get deploy token with ID as argument (project)
			$ glab deploy-token get 1234

			# Get deploy token for a group
			$ glab deploy-token get 1234 --group my-group
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

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Get deploy token for a group instead of a project.")

	return cmd
}

func (o *options) complete(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		tokenID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("deploy token ID must be an integer: %s", args[0])
		}
		o.tokenID = int64(tokenID)
	}

	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	var token *gitlab.DeployToken

	if o.group != "" {
		token, _, err = client.DeployTokens.GetGroupDeployToken(o.group, o.tokenID)
		if err != nil {
			return cmdutils.WrapError(err, "getting deploy token for group.")
		}
	} else {
		baseRepo, err := o.baseRepo()
		if err != nil {
			return err
		}

		token, _, err = client.DeployTokens.GetProjectDeployToken(baseRepo.FullName(), o.tokenID)
		if err != nil {
			return cmdutils.WrapError(err, "getting deploy token for project.")
		}
	}

	if token.ID != 0 {
		table := tableprinter.NewTablePrinter()
		table.AddRow("ID", "Name", "Username", "Scopes", "Expires At", "Revoked", "Expired")
		expiresAt := ""
		if token.ExpiresAt != nil {
			expiresAt = token.ExpiresAt.String()
		}
		table.AddRow(token.ID, token.Name, token.Username, token.Scopes, expiresAt, token.Revoked, token.Expired)
		o.io.LogInfo(table.String())
	} else {
		o.io.LogInfo("Deploy token does not exist.")
	}

	return nil
}
